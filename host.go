package main

import "io"
import "log"
import "os"
import "sort"

type hostData struct {
	Dataset
	JailsCache map[string]int
}

var ZFSRoot = "zroot/zjail"
var Host hostData

var RootProperties = map[string]string{
	"atime":              "off",
	"compress":           "lz4",
	"dedup":              "on",
	"jail:devfs_ruleset": "4",
	"jail:exec.clean":    "true",
	"jail:exec.start":    "/bin/sh /etc/rc",
	"jail:exec.stop":     "/bin/sh /etc/rc.shutdown",
	"jail:interface":     "lo1",
	"jail:mount.devfs":   "true",
	// "mountpoint":         "/srv/jail",
}

func init() {
	Host = hostData{GetDataset(ZFSRoot), nil}
}

func (r hostData) Init(properties map[string]string) error {
	if r.Exists() {
		if err := r.SetProperties(RootProperties); err != nil {
			return err
		}
		if len(properties) > 0 {
			if err := r.SetProperties(properties); err != nil {
				return err
			}
		}
		return r.WriteJailConf()
	} else {
		log.Fatalln("NFY: creating root dataset")
	}
	return nil
}

func (r hostData) Jails() map[string]int {
	if r.JailsCache == nil {
		r.JailsCache = Jls()
	}
	return r.JailsCache
}

type jailsByName []Jail

func (jj jailsByName) Len() int           { return len(jj) }
func (jj jailsByName) Swap(i, j int)      { jj[i], jj[j] = jj[j], jj[i] }
func (jj jailsByName) Less(i, j int) bool { return jj[i].Name < jj[j].Name }

func (r hostData) Jails() []Jail {
	children, err := r.Dataset.Children(0)
	if err != nil {
		log.Fatalln("ERROR:", err)
	}

	rv := make([]Jail, 0, len(children))
	for _, child := range children {
		if child.Type == "filesystem" {
			jail := Jail{Dataset{child}}
			rv = append(rv, jail)
		}
	}

	sort.Sort(jailsByName(rv))
	return rv
}

func (r hostData) Status() {
	for _, child := range r.Jails() {
		child.Status()
	}
}

func (r hostData) WriteConfigTo(w io.Writer) error {
	for _, child := range r.Jails() {
		if err := child.WriteConfigTo(w); err != nil {
			return err
		}
	}

	if f, err := os.Open("/etc/jail.conf.local"); err != nil {
		if !os.IsNotExist(err) {
			log.Println("WARNING:", err)
		}
	} else {
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
	}

	return nil
}

func (r hostData) WriteJailConf() error {
	jailconf, err := os.OpenFile("/etc/.jail.conf.new", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer jailconf.Close()

	err = r.WriteConfigTo(jailconf)
	if err != nil {
		return err
	}
	jailconf.Close()

	err = os.Rename("/etc/jail.conf", "/etc/jail.conf~")
	if err != nil {
		return err
	}

	return os.Rename("/etc/.jail.conf.new", "/etc/jail.conf")
}

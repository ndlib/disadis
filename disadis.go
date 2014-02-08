package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	flag "github.com/ogier/pflag"

	"github.com/dbrower/disadis/disseminator"
)

type Reopener interface {
	Reopen()
}

type loginfo struct {
	name string
	f    *os.File
}

func NewReopener(filename string) *loginfo {
	return &loginfo{name: filename}
}

func (li *loginfo) Reopen() {
	if li.name == "" {
		return
	}
	if li.f != nil {
		log.Println("Reopening Log files")
	}
	newf, err := os.OpenFile(li.name, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(newf)
	if li.f != nil {
		li.f.Close()
	}
	li.f = newf
}

func signalHandler(sig <-chan os.Signal, logw Reopener) {
	for s := range sig {
		log.Println("Got", s)
		switch s {
		case syscall.SIGUSR1:
			logw.Reopen()
		}
	}
}

func main() {
	var (
		port        string
		logfilename string
		logw        Reopener
		pubtktKey   string
		fedoraAddr  string
		prefix      string
	)

	flag.StringVarP(&port, "port", "p", "8080", "port to run on")
	flag.StringVarP(&logfilename, "log", "l", "", "name of log file")
	flag.StringVarP(&pubtktKey, "pubtkt-key", "", "",
		"filename of PEM encoded public key to use for pubtkt authentication")
	flag.StringVarP(&fedoraAddr, "fedora", "", "",
		"url to use for fedora, includes username and password, if needed")
	flag.StringVarP(&prefix, "prefix", "", "",
		"prefix for all fedora id strings. Includes the colon, if there is one")

	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	logw = NewReopener(logfilename)
	logw.Reopen()
	log.Println("-----Starting Server")

	sig := make(chan os.Signal, 5)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGUSR2)
	go signalHandler(sig, logw)

	if fedoraAddr == "" {
		log.Printf("Error: Fedora address must be set. (--fedora <server addr>)")
		os.Exit(1)
	}
	ha := disseminator.NewHydraAuth(fedoraAddr, prefix)
	if pubtktKey != "" {
		ha.CurrentUser = disseminator.NewPubtktAuthFromKeyFile(pubtktKey)
	}
	http.Handle("/d/",
		disseminator.NewDownloadHandler(nil,
			ha,
			disseminator.NewFedoraSource(fedoraAddr, prefix)))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL)
	})
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

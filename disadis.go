package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	gcfg "gopkg.in/gcfg.v1"

	"github.com/ndlib/disadis/auth"
	"github.com/ndlib/disadis/fedora"
)

// A reopener is a log file which knows how to re-open itself.
type reopener interface {
	Reopen()
}

type loginfo struct {
	name string
	f    *os.File
}

func newReopener(filename string) *loginfo {
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

// writePID writes the PID of this process to the file fname.
func writePID(fname string) {
	f, err := os.Create(fname)
	if err != nil {
		log.Printf("Error writing PID to file '%s': %s\n", fname, err.Error())
		return
	}
	pid := os.Getpid()
	fmt.Fprintf(f, "%d", pid)
	f.Close()
}

func signalHandler(sig <-chan os.Signal, logw reopener) {
	for s := range sig {
		log.Println("---Received signal", s)
		switch s {
		case syscall.SIGUSR1:
			logw.Reopen()
		case syscall.SIGINT, syscall.SIGTERM:
			log.Println("Exiting")
			if pidfilename != "" {
				// we don't care if there is an error
				os.Remove(pidfilename)
			}
			os.Exit(1)
		}

	}
}

// the structure of our configuration file.
type config struct {
	General struct {
		Log_filename string
		Fedora_addr  string
		Admin        []string
	}
	Pubtkt struct {
		Key_file string
	}
	Handler map[string]*struct {
		Port          string
		Auth          bool
		Versioned     bool
		Prefix        string
		Datastream    string
		Datastream_id []string
	}
}

var (
	pidfilename string
)

func main() {
	var (
		logfilename string
		logw        reopener
		pubtktKey   string
		fedoraAddr  string
		configFile  string
		config      config
		showVersion bool
	)

	flag.StringVar(&logfilename, "log", "", "name of log file. Defaults to stdout")
	flag.StringVar(&pubtktKey, "pubtkt-key", "",
		"filename of PEM encoded public key to use for pubtkt authentication")
	flag.StringVar(&fedoraAddr, "fedora", "",
		"url to use for fedora, includes username and password, if needed")
	flag.StringVar(&configFile, "config", "",
		"name of config file to use")
	flag.StringVar(&pidfilename, "pid", "", "file to store pid of server")
	flag.BoolVar(&showVersion, "version", false, "Display the version and exit")

	flag.Parse()

	if showVersion {
		fmt.Printf("disadis version %s\n", Version)
		return
	}

	// the config file stuff was grafted onto the command line options
	// this should be made pretty
	if configFile != "" {
		err := gcfg.ReadFileInto(&config, configFile)
		if err != nil {
			log.Println(err)
		}
		logfilename = config.General.Log_filename
		fedoraAddr = config.General.Fedora_addr
		pubtktKey = config.Pubtkt.Key_file
	}

	/* first set up the log file */
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	logw = newReopener(logfilename)
	logw.Reopen()
	log.Println("-----Starting Server")

	/* set up signal handlers */
	sig := make(chan os.Signal, 5)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGUSR2)
	go signalHandler(sig, logw)

	/* Now set up the handler chains */
	if fedoraAddr == "" {
		log.Printf("Error: Fedora address must be set. (--fedora <server addr>)")
		os.Exit(1)
	}
	fedora := fedora.NewRemote(fedoraAddr, "")
	ha := auth.NewHydraAuth(fedoraAddr, "")
	ha.Admin = config.General.Admin
	log.Println("Admin users:", ha.Admin)
	switch {
	case pubtktKey != "":
		log.Printf("Using pubtkt %s", pubtktKey)
		ha.CurrentUser = auth.NewPubtktAuthFromKeyFile(pubtktKey)
	default:
		log.Printf("Warning: No authorization method given.")
	}
	if len(config.Handler) == 0 {
		log.Printf("No Handlers are defined. Exiting.")
		return
	}

	if pidfilename != "" {
		writePID(pidfilename)
	}

	runHandlers(config, fedora, ha)

	if pidfilename != "" {
		os.Remove(pidfilename)
	}
}

type handlerBootstrap struct {
	h    http.Handler
	name string
}

// runHandlers starts a listener for each port in its own goroutine
// and then waits for all of them to quit.
func runHandlers(config config, fedora fedora.Fedora, auth *auth.HydraAuth) {
	var wg sync.WaitGroup
	portHandlers := make(map[string]*DsidMux)
	// first create the handlers
	for k, v := range config.Handler {
		h := &DownloadHandler{
			Fedora:    fedora,
			Ds:        v.Datastream,
			Versioned: v.Versioned,
			Prefix:    v.Prefix,
		}
		if v.Auth {
			h.Auth = auth
		}
		log.Printf("Handler %s (datastream %s, port %s, auth %v, dsid %v)",
			k,
			v.Datastream,
			v.Port,
			v.Auth,
			v.Datastream_id)
		mux, ok := portHandlers[v.Port]
		if !ok {
			mux = &DsidMux{}
			portHandlers[v.Port] = mux
		}
		// see http://golang.org/doc/faq#closures_and_goroutines
		k := k // make local ref to var for closure
		hh := http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				t := time.Now()
				realip := r.Header.Get("X-Real-IP")
				if realip == "" {
					realip = r.RemoteAddr
				}
				h.ServeHTTP(w, r)
				log.Printf("%s %s %s %s %v",
					k,
					realip,
					r.Method,
					r.RequestURI,
					time.Now().Sub(t))
			})
		if len(v.Datastream_id) == 0 {
			mux.DefaultHandler = hh
		}
		for _, name := range v.Datastream_id {
			if name == "default" {
				mux.DefaultHandler = hh
			} else {
				mux.AddHandler(name, hh)
			}
		}
	}
	// now start a goroutine for each port
	for port, h := range portHandlers {
		wg.Add(1)
		go http.ListenAndServe(":"+port, h)
	}
	// Listen on 6060 to get pprof output
	go http.ListenAndServe(":6060", nil)
	// We add things to the waitgroup, but never call wg.Done(). This will never return.
	wg.Wait()
}

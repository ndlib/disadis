package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"code.google.com/p/gcfg"
	_ "github.com/go-sql-driver/mysql"

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
		log.Println("---Got", s)
		switch s {
		case syscall.SIGUSR1:
			logw.Reopen()
		}
	}
}

type Config struct {
	General struct {
		Port string
		Log_filename string
		Fedora_addr string
		Prefix string
	}
	Pubtkt struct {
		Key_file string
	}
	Rails struct {
		Secret string
		Cookie string
		Database string
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
		secret string
		database string
		cookieName string
		config Config
	)

	flag.StringVar(&port, "port", "8080", "port to listen on")
	flag.StringVar(&logfilename, "log", "", "name of log file. Defaults to stdout")
	flag.StringVar(&pubtktKey, "pubtkt-key", "",
		"filename of PEM encoded public key to use for pubtkt authentication")
	flag.StringVar(&fedoraAddr, "fedora", "",
		"url to use for fedora, includes username and password, if needed")
	flag.StringVar(&prefix, "prefix", "",
		"prefix for all fedora id strings. Includes the colon, if there is one")
	flag.StringVar(&secret, "secret", "",
		"secret to use to verify rails 3 cookies")
	flag.StringVar(&database, "db", "",
		"path and credentials to access the user database (mysql). Needed if --secret is given")
	flag.StringVar(&cookieName, "cookie", "",
		"name of cookie holding the rails 3 session")

	flag.Parse()

	// the config file stuff was grafted onto the command line options
	// this should be made pretty
	err := gcfg.ReadFileInto(&config, "settings.ini")
	if err != nil {
		log.Println(err)
	}
	port = config.General.Port
	logfilename = config.General.Log_filename
	fedoraAddr = config.General.Fedora_addr
	prefix = config.General.Prefix
	pubtktKey = config.Pubtkt.Key_file
	secret = config.Rails.Secret
	database = config.Rails.Database
	cookieName = config.Rails.Cookie

	/* first set up the log file */
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	logw = NewReopener(logfilename)
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
	log.Printf("Using prefix '%s'", prefix)
	fedora := disseminator.NewRemoteFedora(fedoraAddr, prefix)
	ha := disseminator.NewHydraAuth(fedoraAddr, prefix)
	switch {
	case pubtktKey != "":
		log.Printf("Using pubtkt %s", pubtktKey)
		ha.CurrentUser = disseminator.NewPubtktAuthFromKeyFile(pubtktKey)
	case secret != "":
		log.Printf("Using Rails 3 cookies")
		if cookieName == "" {
			log.Printf("Warning: The name of the cookie holding the rails session is required (--cookie)")
			break
		}
		log.Printf("Cookie name '%s'", cookieName)
		if database == "" {
			log.Printf("Warning: A database (--db) is required to use rails cookies")
			break
		}
		db, err := sql.Open("mysql", database)
		if err != nil {
			log.Printf("Error opening database connection: %s", err)
			break
		}
		ha.CurrentUser = &disseminator.DeviseAuth{
			SecretBase: []byte(secret),
			CookieName: cookieName,
			Lookup: &disseminator.DatabaseUser{Db: db},
		}
	default:
		log.Printf("Warning: No authorization method given.")
	}
	if ha.CurrentUser == nil {
		log.Printf("Warning: Only Allowing Public Access.")
	}
	/* here is where we would add other handlers to reverse proxy, e.g. */
	ha.Handler = disseminator.NewDownloadHandler(fedora)
	http.Handle("/", ha)

	/* Enter main loop */
	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

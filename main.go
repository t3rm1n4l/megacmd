package main

import (
	"flag"
	"fmt"
	"github.com/t3rm1n4l/go-mega"
	"github.com/t3rm1n4l/megacmd/client"
	"log"
	"os"
	"os/user"
	"path"
	"strings"
)

const (
	CONFIG_FILE = ".megacmd.json"
)

const USAGE = `
	megacmd [OPTIONS] list mega:/foo/bar
	megacmd [OPTIONS] get mega:/foo/file.txt /tmp/
	megacmd [OPTIONS] put /tmp/hello.txt mega:/bar/
	megacmd [OPTIONS] delete mega:/foo/bar
	megacmd [OPTIONS] mkdir mega:/foo/bar
	megacmd [OPTIONS] move mega:/foo/file.txt mega:/bar/foo.txt
	megacmd [OPTIONS] sync mega:/foo/ /tmp/foo/
	megacmd [OPTIONS] sync /tmp/foo mega:/foo

`

const (
	LIST   = "list"
	GET    = "get"
	PUT    = "put"
	DELETE = "delete"
	MKDIR  = "mkdir"
	MOVE   = "move"
	SYNC   = "sync"
)

func main() {
	usr, _ := user.Current()
	var (
		help      = flag.Bool("help", false, "Help")
		version   = flag.Bool("version", false, "Version")
		config    = flag.String("conf", path.Join(usr.HomeDir, CONFIG_FILE), "Config file path")
		recursive = flag.Bool("recursive", false, "Recursive listing")
		force     = flag.Bool("force", false, "Force hard delete or overwrite")
	)

	_ = help
	_ = version
	log.SetFlags(0)

	var Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage %s:", os.Args[0])
		fmt.Fprintf(os.Stderr, USAGE)
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 2 || *help {
		Usage()
		os.Exit(1)
	}

	conf := new(megaclient.Config)
	err := conf.Parse(*config)
	if err != nil {
		log.Fatal(err)
	}

	if *recursive {
		conf.Recursive = true
	}

	if *force {
		conf.Force = true
	}

	client := megaclient.NewMegaClient(conf)
	err = client.Login()
	if err != nil {
		log.Fatal("Login failed, Please verify username or password")
	}

	cmd := flag.Arg(0)
	arg1 := flag.Arg(1)
	arg2 := ""
	if flag.NArg() > 2 {
		arg2 = flag.Arg(2)
	}

	switch {
	case cmd == LIST:
		paths, err := client.List(arg1)
		if err != nil && err != mega.ENOENT {
			log.Fatalf("ERROR: List failed (%s)", err)
		}
		if err == nil {
			for _, p := range *paths {
				log.Println(p)
			}
		}
	case cmd == DELETE:
		err := client.Delete(arg1)
		if err != nil {
			log.Fatalf("ERROR: Unable to delete %s (%s)", arg1, err)
		}
		log.Println("Successfully deleted ", arg1)

	case cmd == MOVE:
		err := client.Move(arg1, arg2)
		if err != nil {
			log.Fatalf("ERROR: Unable to move %s (%s)", arg1, err)
		}

		log.Printf("Successfully moved %s to %s\n", arg1, arg2)

	case cmd == GET:

		if arg2 == "" {
			name := strings.Split(arg1, "/")
			if len(name) > 0 {
				arg2 = name[len(name)-1]
			}
		}

		err := client.Get(arg1, arg2)
		if err != nil {
			log.Fatalf("ERROR: Downloading %s to %s failed (%s)", arg1, arg2, err)
		}
		log.Printf("Successfully downloaded file %s to %s", arg1, arg2)

	case cmd == PUT:
		err := client.Put(arg1, arg2)
		if err != nil {
			log.Fatalf("ERROR: Uploading %s to %s failed (%s)", arg1, arg2, err)
		}

		log.Printf("Successfully uploaded file %s to %s", arg1, arg2)

	case cmd == MKDIR:
		err := client.Mkdir(arg1)
		if err != nil {
			log.Fatalf("ERROR: Unable to create directory %s (%s)", arg1, err)
		}

		log.Printf("Successfully created directory at %s", arg1)

	case cmd == SYNC:
		err := client.Sync(arg1, arg2)
		if err != nil {
			log.Fatalf("ERROR: Unable to sync %s to %s (%s)", arg1, arg2, err)
		}

		log.Printf("Successfully sync %s to %s", arg1, arg2)

	default:
		log.Fatal("Invalid command")
	}

}

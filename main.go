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
)

const (
	CONFIG_FILE = ".megacmd.json"
)

const USAGE = `
	megacmd [OPTIONS] list mega:/foo/bar
	megacmd [OPTIONS] get mega:/foo/file.txt /tmp/
	megacmd [OPTIONS] put /tmp/hello.txt mega:/bar/
	megacmd [OPTIONS] delete mega:/foo/bar
	megacmd [OPTIONS] move mega:/foo/file.txt mega:/bar/foo.txt
	megacmd [OPTIONS] sync mega:/foo/ /tmp/foo/
	megacmd [OPTIONS] sync /tmp/foo mega:/foo

`

const (
	LIST   = "list"
	GET    = "get"
	PUT    = "put"
	DELETE = "delete"
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

	_ = arg2
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
	}

}

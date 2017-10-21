package megaclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/t3rm1n4l/go-mega"
)

const (
	PATH_WIDTH = 50
	SIZE_WIDTH = 10
)

type MegaClient struct {
	cfg  *Config
	mega *mega.Mega
}

type Config struct {
	BaseUrl         string
	Retries         int
	DownloadWorkers int
	UploadWorkers   int
	TimeOut         int
	User            string
	Password        string
	Recursive       bool
	Force           bool
	SkipSameSize    bool
	SkipError       bool
	Verbose         int
}

type Path struct {
	prefix string
	path   []string
	size   int64
	t      int
	ts     time.Time
}

func (p *Path) SetPrefix(s string) {
	p.prefix = s
}

func (p Path) GetPath() string {
	x := path.Join(p.path...)
	x = path.Join(p.prefix, x)
	if p.t == mega.FOLDER {
		x = x + "/"
	}

	return x
}

func (p Path) String() string {
	return fmt.Sprintf("%-*s %-*d %s", PATH_WIDTH, p.GetPath(), SIZE_WIDTH, p.size, p.ts.Format(time.RFC3339))
}

const (
	ROOT  = "mega"
	TRASH = "trash"
)

var (
	EINVALID_CONFIG = errors.New("Invalid json config")
	EINVALID_PATH   = errors.New("Invalid mega path")
	ENOT_FILE       = errors.New("Requested object is not a file")
	EINVALID_DEST   = errors.New("Invalid destination path")
	EINVALID_SRC    = errors.New("Invalid source path")
	EINVALID_SYNC   = errors.New("Invalid sync command parameters")
	ENOT_DIRECTORY  = errors.New("A non-directory exists at this path")
	EFILE_EXISTS    = errors.New("File with same name already exists")
	EDIR_EXISTS     = errors.New("A directory with same name already exists")
)

func (cfg *Config) Parse(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, cfg)

	if err != nil {
		return EINVALID_CONFIG
	}

	return nil
}

func NewMegaClient(conf *Config) (*MegaClient, error) {
	log.SetFlags(0)
	var err error
	c := &MegaClient{
		cfg:  conf,
		mega: mega.New(),
	}

	if conf.BaseUrl != "" {
		c.mega.SetAPIUrl(conf.BaseUrl)
	}

	if conf.Retries != 0 {
		c.mega.SetRetries(conf.Retries)
	}

	if conf.DownloadWorkers != 0 {
		err = c.mega.SetDownloadWorkers(conf.DownloadWorkers)

		if err == mega.EWORKER_LIMIT_EXCEEDED {
			err = errors.New(fmt.Sprintf("%s : %d <= %d", err, conf.DownloadWorkers, mega.MAX_DOWNLOAD_WORKERS))
		}
	}

	if conf.UploadWorkers != 0 {
		err = c.mega.SetUploadWorkers(conf.UploadWorkers)
		if err == mega.EWORKER_LIMIT_EXCEEDED {
			err = errors.New(fmt.Sprintf("%s : %d <= %d", err, conf.DownloadWorkers, mega.MAX_UPLOAD_WORKERS))
		}
	}

	if conf.TimeOut != 0 {
		c.mega.SetTimeOut(time.Duration(conf.TimeOut) * time.Second)
	}

	return c, err
}

func (mc *MegaClient) Login() error {
	err := mc.mega.Login(mc.cfg.User, mc.cfg.Password)
	return err
}

func (mc *MegaClient) List(resource string) (*[]Path, error) {
	var root *mega.Node
	var paths []Path
	var err error

	root, pathsplit, err := getLookupParams(resource, mc.mega.FS)
	if err != nil {
		return nil, err
	}

	var nodes []*mega.Node
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	}

	if err == nil {
		l := len(nodes)
		var indexnode *mega.Node
		switch {
		case len(*pathsplit) == 0:
			indexnode = root
			nodes, _ = mc.mega.FS.GetChildren(root)
			if err != nil {
				return nil, err
			}
		case l > 0:
			indexnode = nodes[l-1]
			nodes, _ = mc.mega.FS.GetChildren(nodes[l-1])
			if err != nil {
				return nil, err
			}
		}

		if indexnode != nil && strings.HasSuffix(resource, "/") == false {
			var p Path
			p.SetPrefix(resource)
			p.t = indexnode.GetType()
			p.size = indexnode.GetSize()
			p.ts = indexnode.GetTimeStamp()
			paths = append(paths, p)
		} else {
			for _, n := range nodes {
				for _, p := range getRemotePaths(mc.mega.FS, n, mc.cfg.Recursive) {
					p.SetPrefix(resource)
					paths = append(paths, p)
				}
			}
		}

		return &paths, nil
	}

	return nil, err
}

func (mc *MegaClient) Delete(resource string) error {
	root, pathsplit, err := getLookupParams(resource, mc.mega.FS)
	if err != nil {
		return err
	}

	var nodes []*mega.Node
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		err = EINVALID_PATH
	}

	if err != nil {
		return err
	}

	l := len(nodes)
	node := nodes[l-1]

	return mc.mega.Delete(node, mc.cfg.Force)
}

func (mc *MegaClient) Move(srcres, dstres string) error {
	root, pathsplit, err := getLookupParams(srcres, mc.mega.FS)
	if err != nil {
		return err
	}

	var nodes []*mega.Node
	var srcnode, dstnode *mega.Node
	var name string
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		err = EINVALID_PATH
	}

	if err != nil {
		return err
	}

	srcnode = nodes[len(nodes)-1]

	root, pathsplit, err = getLookupParams(dstres, mc.mega.FS)
	if err != nil {
		return err
	}

	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		err = EINVALID_PATH
	}

	if err != nil && err != mega.ENOENT {
		return err
	}

	lp := len(*pathsplit)
	ln := len(nodes)

	var rename bool
	switch {
	case lp == ln:
		dstnode = nodes[ln-1]
		if strings.HasSuffix(dstres, "/") {
			if dstnode.GetType() != mega.FOLDER {
				return EFILE_EXISTS
			}
		} else {
			if dstnode.GetType() == mega.FOLDER {
				return EDIR_EXISTS
			} else {
				return EFILE_EXISTS
			}
		}
		rename = false
	case lp == ln+1:
		if ln == 0 {
			dstnode = root
		} else {
			dstnode = nodes[ln-1]
		}
		name = (*pathsplit)[lp-1]
		rename = true
	default:
		return err
	}

	err = mc.mega.Move(srcnode, dstnode)

	if err != nil {
		return err
	}

	if rename {
		err = mc.mega.Rename(srcnode, name)
	}

	return err
}

func (mc *MegaClient) Get(srcres, dstpath string) error {
	root, pathsplit, err := getLookupParams(srcres, mc.mega.FS)
	if err != nil {
		return err
	}

	var nodes []*mega.Node
	var node *mega.Node

	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		err = EINVALID_PATH
	}

	if err != nil {
		return err
	} else {
		node = nodes[len(nodes)-1]
		if node.GetType() != mega.FILE {
			return ENOT_FILE
		}
	}

	fi, err := os.Stat(dstpath)
	if os.IsNotExist(err) {
		d := path.Dir(dstpath)
		fi, err := os.Stat(d)
		if os.IsNotExist(err) {
			return EINVALID_DEST
		} else {
			if !fi.Mode().IsDir() {
				return EINVALID_DEST
			}
		}
	} else {
		if fi.Mode().IsDir() {
			if strings.HasSuffix(dstpath, "/") {
				dstpath = path.Join(dstpath, (*pathsplit)[len(*pathsplit)-1])
			} else {
				return EDIR_EXISTS
			}
		}

		info, err := os.Stat(dstpath)
		if os.IsNotExist(err) == false {

			if mc.cfg.SkipSameSize && info.Size() == node.GetSize() {
				return nil
			}

			if mc.cfg.Force {
				err = os.Remove(dstpath)
				if err != nil {
					return err
				}
			} else {
				return EFILE_EXISTS
			}
		}
		err = nil
	}

	var ch *chan int
	var wg sync.WaitGroup
	if mc.cfg.Verbose > 0 {
		ch = new(chan int)
		*ch = make(chan int)

		wg.Add(1)
		go progressBar(*ch, &wg, node.GetSize(), srcres, dstpath)
	}

	err = mc.mega.DownloadFile(node, dstpath, ch)
	wg.Wait()
	return err
}

func (mc *MegaClient) Put(srcpath, dstres string) error {
	var nodes []*mega.Node
	var node *mega.Node
	info, err := os.Stat(srcpath)

	if err != nil {
		return EINVALID_SRC
	}

	if info.Mode()&os.ModeType != 0 {
		return ENOT_FILE
	}

	root, pathsplit, err := getLookupParams(dstres, mc.mega.FS)
	if err != nil {
		return err
	}
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	}

	if err != nil && err != mega.ENOENT {
		return err
	}

	lp := len(*pathsplit)
	ln := len(nodes)

	var name string

	switch {

	case lp == ln+1 && ln > 0:
		node = nodes[ln-1]
		if node.GetType() == mega.FOLDER && strings.HasSuffix(dstres, "/") == false {
			name = (*pathsplit)[lp-1]
		} else {
			return err
		}
	case lp == ln:
		name = path.Base(srcpath)
		if lp == 0 {
			node = root
		} else {
			node = nodes[ln-1]
			if node.GetType() == mega.FOLDER {
				if strings.HasSuffix(dstres, "/") == false {
					return EDIR_EXISTS
				}
			} else {
				if strings.HasSuffix(dstres, "/") == true {
					return ENOT_DIRECTORY
				}
				name = path.Base(dstres)
				if len(nodes) > 1 {
					node = nodes[ln-2]
				} else {
					node = root
				}
			}
		}
	case ln == 0 && lp == 1:
		if strings.HasSuffix(dstres, "/") == false {
			node = root
			name = path.Base(srcpath)
		} else {
			return err
		}
	default:
		return err
	}

	children, err := mc.mega.FS.GetChildren(node)
	if err != nil {
		return err
	}

	for _, c := range children {
		if c.GetName() == name {
			if mc.cfg.SkipSameSize && info.Size() == c.GetSize() {
				return nil
			}

			if mc.cfg.Force {
				err = mc.mega.Delete(c, false)
				if err != nil {
					return err
				}
				if err != nil {
					return err
				}
			} else {
				return EFILE_EXISTS
			}
		}
	}

	var ch *chan int
	var wg sync.WaitGroup
	if mc.cfg.Verbose > 0 {
		ch = new(chan int)
		*ch = make(chan int)
		fi, err := os.Stat(srcpath)
		if err != nil {
			return err
		}

		wg.Add(1)
		go progressBar(*ch, &wg, fi.Size(), srcpath, dstres)
	}

	_, err = mc.mega.UploadFile(srcpath, node, name, ch)
	wg.Wait()
	return err
}

func (mc *MegaClient) Mkdir(dstres string) error {
	var nodes []*mega.Node
	var node *mega.Node

	root, pathsplit, err := getLookupParams(dstres, mc.mega.FS)
	if err != nil {
		return err
	}
	if len(*pathsplit) > 0 {
		nodes, err = mc.mega.FS.PathLookup(root, *pathsplit)
	} else {
		return nil
	}

	lp := len(*pathsplit)
	ln := len(nodes)

	if len(nodes) > 0 {
		node = nodes[ln-1]
	} else {
		node = root
	}

	switch {
	case err == nil:
		if node.GetType() != mega.FOLDER {
			return ENOT_DIRECTORY
		}
		return nil
	case err == mega.ENOENT:
		remaining := lp - ln
		for i := 0; i < remaining; i++ {
			name := (*pathsplit)[ln]
			node, err = mc.mega.CreateDir(name, node)
			if err != nil {
				return err
			}
			ln += 1
		}
		err = nil

	default:
		return err
	}

	return nil
}

func (mc *MegaClient) Sync(src, dst string) error {
	var srcremote bool
	var paths []Path
	root, pathsplits, err := getLookupParams(src, mc.mega.FS)
	switch {
	case err == EINVALID_PATH:
		r, ps, e := getLookupParams(dst, mc.mega.FS)
		if e != nil {
			return EINVALID_SYNC
		}
		root = r
		pathsplits = ps
		err = e
		srcremote = false
	case err == nil:
		_, _, e := getLookupParams(dst, mc.mega.FS)
		if e != EINVALID_PATH {
			return EINVALID_SYNC
		}
		err = e
		srcremote = true
	default:
		return err
	}

	if srcremote {
		var node *mega.Node
		nodes, err := mc.mega.FS.PathLookup(root, *pathsplits)
		if err != nil {
			return err
		}
		if len(nodes) > 0 {
			node = nodes[len(nodes)-1]
		} else {
			node = root
		}

		children, err := mc.mega.FS.GetChildren(node)
		if err != nil {
			return err
		}

		for _, n := range children {
			paths = append(paths, getRemotePaths(mc.mega.FS, n, true)...)
		}
	} else {
		paths, err = getLocalPaths(src, mc.cfg.SkipError)
		if err != nil {
			return err
		}
	}

	if mc.cfg.Verbose > 0 {
		log.Printf("Found %d file(s) to be copied", len(paths))
	}

	for _, spath := range paths {
		suffix := spath.GetPath()
		x := path.Join(src, suffix)
		y := path.Join(dst, suffix)

		dir := y
		if spath.t == mega.FILE {
			dir = path.Dir(y)
		}

		if srcremote {
			err = os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				return err
			}
			if spath.t == mega.FILE {
				err = mc.Get(x, y)
			}
		} else {
			err = mc.Mkdir(dir)
			if err != nil {
				return err
			}

			if err != nil {
				return err
			}

			if spath.t == mega.FILE {
				err = mc.Put(x, y)
			}
		}
		if mc.cfg.Verbose > 0 {
			if err == EFILE_EXISTS {
				file := path.Join(dst, spath.GetPath())
				err = errors.New(fmt.Sprintf("%s - %s", file, EFILE_EXISTS))
			}
		}

		if err != nil {
			return err
		}
	}

	return nil
}

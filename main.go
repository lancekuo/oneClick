package main

import (
	"encoding/json"
	"fmt"
	"github.com/bugagazavr/go-gitlab-client"
	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/render"
	"io"
	"io/ioutil"
	"net"
	"os"
	"runtime"
	"time"
)

const DEBUG bool = false
const SOURCEPATH string = `Z:%s\LMPI\`
const VERSIONFILE string = `\version.txt`
const TARGETPATH string = `D:%s\LMPI`

type Config struct {
	Host        string `json:"host"`
	ApiPath     string `json:"api_path"`
	Token       string `json:"token"`
	Project     string `json:"project_id"`
	Environment string `json:"environment"`
}

var config Config
var target_path string
var source_path string
var oneclick map[string]string

func getIPAddr() (IPAddr []string) {

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		os.Stderr.WriteString("Oops: " + err.Error() + "<br>")
		os.Exit(1)
	}

	for _, a := range addrs {
		switch runtime.GOOS {
		case "windows":
			ipnet, ok := a.(*net.IPAddr)
			if ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					IPAddr = append(IPAddr, ipnet.IP.String())
				}
			}
		default:
			ipnet, ok := a.(*net.IPNet)
			if ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					IPAddr = append(IPAddr, ipnet.IP.String())
				}
			}
		}
	}
	return
}
func CopyFile(source string, dest string) (err error) {
	sourcefile, err := os.Open(source)
	if err != nil {
		return err
	}

	defer sourcefile.Close()

	destfile, err := os.Create(dest)
	if err != nil {
		return err
	}

	defer destfile.Close()

	_, err = io.Copy(destfile, sourcefile)
	if err == nil {
		sourceinfo, err := os.Stat(source)
		if err != nil {
			err = os.Chmod(dest, sourceinfo.Mode())
		}

	}

	return
}

func CopyDir(source string, dest string) (err error) {

	// get properties of source dir
	sourceinfo, err := os.Stat(source)
	if err != nil {
		return err
	}

	// create dest dir

	err = os.MkdirAll(dest, sourceinfo.Mode())
	if err != nil {
		return err
	}

	directory, _ := os.Open(source)

	objects, err := directory.Readdir(-1)

	for _, obj := range objects {

		sourcefilepointer := source + "/" + obj.Name()

		destinationfilepointer := dest + "/" + obj.Name()

		if obj.IsDir() {
			// create sub-directories - recursively
			err = CopyDir(sourcefilepointer, destinationfilepointer)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			// perform copy
			err = CopyFile(sourcefilepointer, destinationfilepointer)
			if err != nil {
				fmt.Println(err)
			}
		}

	}
	return
}

func getTags() []*gogitlab.Tag {

	gitlab := gogitlab.NewGitlabCert(config.Host, config.ApiPath, config.Token, true)

	fmt.Println("Fetching repository tags…")
	tags, err := gitlab.RepoTags(config.Project)
	if err != nil {
		fmt.Println(err.Error())
	}

	return tags
}

func getCurrVersion() string {
	fmt.Println("Start getCurrVersion")
	data := ""
	ext := `\Inetpub`
	if DEBUG {
		ext = `\www`
	}
	target_path = fmt.Sprintf(TARGETPATH, ext)
	file, e := ioutil.ReadFile(target_path + VERSIONFILE)
	if e != nil {
		fmt.Printf("==getCurrVersion== CurrentVersion not found: %v\n", e)
		data = "No data"
	} else {
		data = string(file)
	}

	fmt.Println("End getCurrVersion")
	return data
}

func getRemoteBin() string {
	ext := ""
	if DEBUG {
		ext = "\\lancek"
	}
	source_path = fmt.Sprintf(SOURCEPATH, ext)

	return source_path
}

func retrieveTagList() []*gogitlab.Tag {
	fmt.Printf("We are at: %s\n", config.Environment)

	oneclick = make(map[string]string)

	tags := getTags()
	path := getRemoteBin()
	for _, tag := range tags {
		file, e := ioutil.ReadFile(path + tag.Name + VERSIONFILE)
		if e != nil {
			fmt.Printf("Build not found: %v\n", e)
			oneclick[tag.Name] = ""
		} else {
			oneclick[tag.Name] = string(file)
		}
	}

	return tags

}

func updateFolder(ver string) {

	config_path := source_path + `Config\` + config.Environment + `\`
	source_path += ver
	fmt.Printf("==UpdateFolder== Source: %s, Target: %s\n", source_path, target_path)

	err := CopyDir(source_path, target_path)
	if err != nil {
		fmt.Println(err)
	}

	err = CopyDir(config_path, target_path)
	if err != nil {
		fmt.Println(err)
	}

}

func main() {
	startedAt := time.Now()
	defer func() {
		fmt.Printf("processed in %v\n", time.Now().Sub(startedAt))
	}()

	file, e := ioutil.ReadFile("config.json")
	if e != nil {
		fmt.Printf("Config file error: %v\n", e)
		os.Exit(1)
	}

	json.Unmarshal(file, &config)

	fmt.Printf("Results: %+v\n", config)

	m := martini.Classic()
	m.Use(render.Renderer())
	m.Get("/", func(r render.Render) {
		addrs := getIPAddr()
		tags := retrieveTagList()
		//for _, tag := range tags {
		//		fmt.Printf("> Version: %s, Message: %s, FileExist: %s\n", tag.Name, tag.Commit.Message, oneclick[tag.Name])
		//	}

		current_version := getCurrVersion()

		data := struct {
			CurrVersion string
			Environment string
			IPAddr      []string
			Tags        []*gogitlab.Tag
			FileStatus  map[string]string
		}{
			current_version,
			config.Environment,
			addrs,
			tags,
			oneclick,
		}

		r.HTML(200, "home", data)
	})
	m.Get("/update/:ver", func(r render.Render, params martini.Params) {
		updateFolder(params["ver"])
		r.Redirect("/")
	})
	m.Run()
}

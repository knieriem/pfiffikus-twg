package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/knieriem/asset"
	"github.com/knieriem/text/ini"

	"github.com/russross/blackfriday/v2"
)

type pageSpec struct {
	Name  string
	Title string
}

type pageData struct {
	Current *pageSpec
	Nav     []*pageSpec
	Body    template.HTML
}

type Person struct {
	GivenName          string
	Surname            string
	Position           string
	PositionAlt        string
	Since              string
	Where              string
	ExtraQualification string
	Phone              struct {
		Mobile string
		When   string
	}
}

func (p *Person) FullName() string {
	return p.GivenName + " " + p.Surname
}

type conf struct {
	ServiceAddr string
	AccessAddr  string
	Person      map[string]*Person
}

var c conf

func main() {
	flag.Parse()

	cmd := flag.Arg(0)
	if cmd == "" {
		cmd = "serve"
	}

	asset.BindExeDir()
	ini.BindFS(asset.FS)
	fd := ini.NewFile("data.txt", ".txt", "data")
	err := fd.Parse(&c)
	if err != nil {
		log.Fatal(err)
	}
	b := new(Builder)

	var run func(*Builder) error

	switch cmd {
	case "build":
		forceRmdir := flag.Arg(1) == "-f"
		run = func(b *Builder) error {
			return buildSite(b, forceRmdir)
		}
	case "serve":
		run = func(b *Builder) error {
			return serve(b, c.ServiceAddr, c.AccessAddr)
		}
	default:
		fmt.Println("usage: twg {build|serve}")
		os.Exit(2)
	}

	pageList, err := parseNavigation("navigation.txt")
	if err != nil {
		log.Fatal(err)
	}
	t, err := template.New("main").ParseFiles("template.html")
	if err != nil {
		log.Fatal(err)
	}
	t.Funcs(template.FuncMap{
		"person": func(key string) *Person {
			if c.Person == nil {
				fmt.Println("* missing person for key:", key)
				return &Person{GivenName: "MISSING: " + strconv.Quote(key)}
			}
			p, ok := c.Person[key]
			if !ok {
				fmt.Println("* missing person for key:", key)
				return &Person{GivenName: "MISSING: " + strconv.Quote(key)}
			}
			return p
		},
	})
	b.pageList = pageList
	b.t = t

	err = run(b)
	if err != nil {
		log.Fatal(err)
	}
}

func parseNavigation(filename string) ([]*pageSpec, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	var list []*pageSpec
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(line)
		f := strings.SplitN(line, " ", 2)
		if len(f) != 2 {
			continue
		}
		list = append(list, &pageSpec{Name: f[0], Title: f[1]})
	}
	return list, nil
}

type Builder struct {
	t        *template.Template
	pageList []*pageSpec
}

func buildSite(b *Builder, forceRmdir bool) error {
	if forceRmdir {
		err := os.RemoveAll("public")
		if err != nil {
			return err
		}
	}
	err := cp("static", "public")
	if err != nil {
		return err
	}

	var out bytes.Buffer
	for _, p := range b.pageList {
		filename := p.Name + ".html"
		out.Reset()
		otime, err := b.buildPage(&out, p)
		if err != nil {
			return err
		}
		target := filepath.Join("public", filename)
		w, err := os.Create(target)
		if err != nil {
			return err
		}
		_, err = out.WriteTo(w)
		w.Close()
		if err != nil {
			return err
		}
		err = os.Chtimes(target, *otime, *otime)
		if err != nil {
			return err
		}

	}
	return nil
}

func serve(b *Builder, addr, url string) error {
	var hIndex http.HandlerFunc
	for i := range b.pageList {
		p := b.pageList[i]
		h := func(w http.ResponseWriter, req *http.Request) {
			var outb bytes.Buffer
			_, err := b.buildPage(&outb, p)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			outb.WriteTo(w)
		}
		if p.Name == "index" {
			hIndex = h
		}
		http.HandleFunc("/"+p.Name+".html", h)
	}
	hfs := http.FileServer(http.Dir("static"))
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			if hIndex != nil {
				hIndex(w, req)
			} else {
				http.NotFound(w, req)
			}
		} else {
			hfs.ServeHTTP(w, req)
		}
	})
	fmt.Println(url)
	return http.ListenAndServe(addr, nil)
}

func (b *Builder) buildPage(w io.Writer, p *pageSpec) (*time.Time, error) {
	input, otime, err := parseContentFile(p.Name)
	if err != nil {
		return nil, err
	}
	d := new(pageData)
	d.Nav = b.pageList
	d.Current = p
	d.Body = template.HTML(input)
	return otime, b.t.ExecuteTemplate(w, "template.html", d)
}

func parseContentFile(stem string) (string, *time.Time, error) {
	ext := ".md"
retry:
	filename := stem + ext
	srcFilename := filepath.Join("content", filename)
	f, err := os.Open(srcFilename)
	if err != nil {
		if ext != ".html" {
			ext = ".html"
			goto retry
		}
		return "", nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return "", nil, err
	}
	otime := fi.ModTime()
	fmt.Println("* create content", strconv.Quote(stem+ext))
	r := bufio.NewScanner(f)
	var input string
	for r.Scan() {
		line := r.Text()
		if strings.HasPrefix(line, "//") {
			continue
		}
		input += line + "\n"
	}
	f.Close()
	t := template.New("content")
	t.Funcs(template.FuncMap{
		"person": func(key string) *Person {
			if c.Person == nil {
				fmt.Println("* missing person for key:", key)
				return &Person{GivenName: "MISSING: " + strconv.Quote(key)}
			}
			p, ok := c.Person[key]
			if !ok {
				fmt.Println("* missing person for key:", key)
				return &Person{GivenName: "MISSING: " + strconv.Quote(key)}
			}
			return p
		},
	})
	t, err = t.Parse(input)
	if err != nil {
		return "", nil, err
	}
	var b bytes.Buffer
	err = t.Execute(&b, &c)
	if err != nil {
		return "", nil, err
	}
	page := b.Bytes()
	if ext == ".md" {
		page = blackfriday.Run(page)
	}
	return string(page), &otime, nil
}

func cp(from, to string) error {
	err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := "public"
		if relPath != "" {
			target = filepath.Join(target, relPath)
		}
		mtime := info.ModTime()
		if info.IsDir() {
			err = os.Mkdir(target, 0755)
			if err != nil {
				return err
			}
		} else {
			b, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			err = ioutil.WriteFile(target, b, 0644)
			if err != nil {
				return err
			}
			err = os.Chtimes(target, mtime, mtime)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := "public"
		if relPath != "" {
			target = filepath.Join(target, relPath)
		}
		mtime := info.ModTime()
		if info.IsDir() {
			err := os.Chtimes(target, mtime, mtime)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

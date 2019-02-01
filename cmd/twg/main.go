package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/knieriem/asset"
	"github.com/knieriem/text/ini"
)

type pageSpec struct {
	Name  string
	Title string
}

type pageData struct {
	Current *pageSpec
	Nav     []*pageSpec
	Body    template.HTML
	Person  map[string]*Person
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
	Person map[string]*Person
}

var c conf

func main() {
	f, err := os.Open("navigation.txt")
	if err != nil {
		log.Fatal(err)
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
	t, err := template.New("main").ParseFiles("template.html")
	if err != nil {
		log.Fatal(err)
	}
	asset.BindExeDir()
	fd := ini.NewFile("data.txt", ".txt", "data")
	ini.BindFS(asset.FS)
	err = fd.Parse(&c)
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range list {
		filename := p.Name + ".html"
		input, err := parseContentFile(p.Name)
		if err != nil {
			log.Fatal(err)
		}
		w, err := os.Create(filepath.Join("public", filename))
		if err != nil {
			log.Fatal(err)
		}
		d := new(pageData)
		d.Nav = list
		d.Current = p
		d.Body = template.HTML(input)
		d.Person = c.Person
		err = t.ExecuteTemplate(w, "template.html", d)
		w.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
}

func parseContentFile(stem string) (string, error) {
	fmt.Println("* create content", strconv.Quote(stem))
	filename := stem + ".html"
	f, err := os.Open(filepath.Join("content", filename))
	if err != nil {
		return "", err
	}
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
	t, err := template.New("content").Parse(input)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	err = t.Execute(&b, &c)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

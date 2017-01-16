package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/ponzu-cms/ponzu/system/admin"
	"github.com/ponzu-cms/ponzu/system/api"
	"github.com/ponzu-cms/ponzu/system/api/analytics"
	"github.com/ponzu-cms/ponzu/system/db"
	"github.com/ponzu-cms/ponzu/system/tls"

	_ "github.com/ponzu-cms/ponzu/content"
)

var (
	usage     = usageHeader + usageNew + usageGenerate + usageBuild + usageRun
	port      int
	httpsport int
	https     bool
	devhttps  bool

	// for ponzu internal / core development
	dev   bool
	fork  string
	gocmd string
)

func main() {
	flag.Usage = func() {
		fmt.Println(usage)
	}

	flag.IntVar(&port, "port", 8080, "port for ponzu to bind its HTTP listener")
	flag.IntVar(&httpsport, "httpsport", 443, "port for ponzu to bind its HTTPS listener")
	flag.BoolVar(&https, "https", false, "enable automatic TLS/SSL certificate management")
	flag.BoolVar(&devhttps, "devhttps", false, "[dev environment] enable automatic TLS/SSL certificate management")
	flag.BoolVar(&dev, "dev", false, "modify environment for Ponzu core development")
	flag.StringVar(&fork, "fork", "", "modify repo source for Ponzu core development")
	flag.StringVar(&gocmd, "gocmd", "go", "custom go command if using beta or new release of Go")
	flag.Parse()

	args := flag.Args()

	if len(args) < 1 {
		fmt.Println(usage)
		os.Exit(0)
	}

	switch args[0] {
	case "help", "h":
		if len(args) < 2 {
			fmt.Println(usageHelp)
			fmt.Println(usage)
			os.Exit(0)
		}

		switch args[1] {
		case "new":
			fmt.Println(usageNew)
			os.Exit(0)

		case "generate", "gen", "g":
			fmt.Println(usageGenerate)
			os.Exit(0)

		case "build":
			fmt.Println(usageBuild)
			os.Exit(0)

		case "run":
			fmt.Println(usageRun)
			os.Exit(0)
		}

	case "new":
		if len(args) < 2 {
			fmt.Println(usageNew)
			os.Exit(0)
		}

		err := newProjectInDir(args[1])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "generate", "gen", "g":
		if len(args) < 3 {
			fmt.Println(usageGenerate)
			os.Exit(0)
		}

		// check what we are asked to generate
		switch args[1] {
		case "content", "c":
			err := generateContentType(args[2:])
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		default:
			msg := fmt.Sprintf("Generator '%s' is not implemented.", args[1])
			fmt.Println(msg)
		}

	case "build":
		err := buildPonzuServer(args)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "run":
		fmt.Println("Running..")
		var addTLS string
		if https {
			addTLS = "--https"
		} else {
			addTLS = "--https=false"
		}

		if devhttps {
			addTLS = "--devhttps"
		}

		var services string
		if len(args) > 1 {
			services = args[1]
		} else {
			services = "admin,api"
		}

		fmt.Println("services:", services)

		serve := exec.Command("./ponzu-server",
			fmt.Sprintf("--port=%d", port),
			fmt.Sprintf("--httpsport=%d", httpsport),
			addTLS,
			"serve",
			services,
		)
		serve.Stderr = os.Stderr
		serve.Stdout = os.Stdout

		err := serve.Start()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		err = serve.Wait()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println("serve command executed.")

	case "serve", "s":
		db.Init()
		defer db.Close()
		fmt.Println("called db.Init()")

		analytics.Init()
		defer analytics.Close()
		fmt.Println("called analytics.Init()")

		if len(args) > 1 {
			services := strings.Split(args[1], ",")
			fmt.Println("configured to start services:", services)

			for i := range services {
				if services[i] == "api" {
					api.Run()
					fmt.Println("called api.Run()")

				} else if services[i] == "admin" {
					admin.Run()
					fmt.Println("called admin.Run()")

				} else {
					fmt.Println("To execute 'ponzu serve', you must specify which service to run.")
					fmt.Println("$ ponzu --help")
					os.Exit(1)
				}
			}
		}

		// save the https port the system is listening on
		err := db.PutConfig("https_port", fmt.Sprintf("%d", httpsport))
		if err != nil {
			log.Fatalln("System failed to save config. Please try to run again.", err)
		}
		fmt.Println("called db.PutConfig('https_port')")

		// cannot run production HTTPS and development HTTPS together
		if devhttps {
			fmt.Println("Enabling self-signed HTTPS... [DEV]")

			go tls.EnableDev()
			fmt.Println("Server listening on https://localhost:10443 for requests... [DEV]")
			fmt.Println("----")
			fmt.Println("If your browser rejects HTTPS requests, try allowing insecure connections on localhost.")
			fmt.Println("on Chrome, visit chrome://flags/#allow-insecure-localhost")

		} else if https {
			fmt.Println("Enabling HTTPS...")

			go tls.Enable()
			fmt.Printf("Server listening on :%s for HTTPS requests...\n", db.ConfigCache("https_port").(string))
		}

		// save the https port the system is listening on so internal system can make
		// HTTP api calls while in dev or production w/o adding more cli flags
		err = db.PutConfig("http_port", fmt.Sprintf("%d", port))
		if err != nil {
			log.Fatalln("System failed to save config. Please try to run again.", err)
		}
		fmt.Println("called db.PutConfig('http_port')")

		log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
		fmt.Println("called http.ListenAndServe()")

	case "":
		fmt.Println(usage)
		fmt.Println(usageHelp)

	default:
		fmt.Println(usage)
		fmt.Println(usageHelp)
	}
}

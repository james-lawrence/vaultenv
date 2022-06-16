package main

import (
	"bytes"
	"io"
	"log"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/james-lawrence/vaultenv"
)

func main() {
	var (
		err     error
		path    string
		secrets []string
		clean   bool
		v       vaultenv.Vault
		b       = bytes.NewBufferString("")
	)

	app := kingpin.New("vaultenv", "converts vault secrets to environment variables")
	app.Arg("secrets", "paths to secrets being read").StringsVar(&secrets)
	app.Flag("clean", "clear the environment before merging secrets").Default("false").BoolVar(&clean)
	app.Flag("output", "set the destination file to output; defaults to stdout").Short('o').StringVar(&path)
	if _, err = app.Parse(os.Args[1:]); err != nil {
		log.Fatalln(err)
		return
	}

	if v, err = vaultenv.NewVault(); err != nil {
		log.Fatalln(err)
		return
	}

	if clean {
		os.Clearenv()
	}

	for _, s := range secrets {
		if err = v.Read(s); err != nil {
			log.Fatalln(err)
		}
	}

	for _, env := range os.Environ() {
		if _, err = b.WriteString(env); err != nil {
			log.Fatalln(err)
		}

		b.WriteString("\n")
	}

	var out io.Writer = os.Stdout
	if len(path) > 0 {
		if dst, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600); err != nil {
			log.Fatalln(err)
		} else {
			out = dst
		}
	}

	if _, err = io.Copy(out, b); err != nil {
		log.Fatalln(err)
	}
}

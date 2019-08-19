package main

import (
	"log"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/james-lawrence/vaultenv"
)

func main() {
	var (
		err     error
		secrets []string
		clean   bool
		v       vaultenv.Vault
	)

	app := kingpin.New("vaultenv", "converts vault secrets to environment variables")
	app.Arg("secrets", "paths to secrets being read").StringsVar(&secrets)
	app.Flag("clean", "clear the environment before merging secrets").Default("false").BoolVar(&clean)

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
		if _, err = os.Stdout.WriteString(env); err != nil {
			log.Fatalln(err)
		}
		os.Stdout.WriteString("\n")
	}
}

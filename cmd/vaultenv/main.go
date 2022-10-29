package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/alecthomas/kingpin"
	"github.com/james-lawrence/vaultenv"
)

func envcmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	var (
		err     error
		path    string
		secrets []string
		clean   bool
		v       vaultenv.Vault
		b       = bytes.NewBufferString("")
	)

	cmd.Arg("secrets", "paths to secrets being read").StringsVar(&secrets)
	cmd.Flag("clean", "clear the environment before merging secrets").Default("false").BoolVar(&clean)
	cmd.Flag("output", "set the destination file to output; defaults to stdout").Short('o').StringVar(&path)

	cmd.Action(func(pc *kingpin.ParseContext) error {
		if v, err = vaultenv.NewVault(vaultenv.DetectAuth()); err != nil {
			return err
		}

		if clean {
			os.Clearenv()
		}

		for _, s := range secrets {
			if err = v.Read(s); err != nil {
				return err
			}
		}

		for _, env := range os.Environ() {
			if _, err = b.WriteString(env); err != nil {
				return err
			}

			b.WriteString("\n")
		}

		var out io.Writer = os.Stdout
		if len(path) > 0 {
			if dst, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600); err != nil {
				return err
			} else {
				out = dst
			}
		}

		if _, err = io.Copy(out, b); err != nil {
			return err
		}

		return nil
	})

	return cmd
}

func execcmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	var (
		err     error
		secrets []string
		clean   bool
		v       vaultenv.Vault
	)

	cmd.Arg("secrets", "paths to secrets and the command to execute separated by a ':'. i.e.) vaultenv exec my/secret/path : echo hello world").StringsVar(&secrets)
	cmd.Flag("clean", "clear the environment before merging secrets").Default("false").BoolVar(&clean)
	cmd.Action(func(pc *kingpin.ParseContext) error {
		if v, err = vaultenv.NewVault(vaultenv.DetectAuth()); err != nil {
			return err
		}

		resolvable := make([]string, 0, len(secrets))
		cmd := make([]string, 0, len(secrets))
		args := false

		for _, s := range secrets {
			if args {
				cmd = append(cmd, s)
				continue
			}

			if s == ":" {
				args = true
				continue
			}

			resolvable = append(resolvable, s)
		}

		if len(cmd) == 0 {
			return errors.New("command required")
		}

		bin, argv := cmd[0], cmd[0:]

		if bin, err = exec.LookPath(bin); err != nil {
			return err
		}

		if clean {
			os.Clearenv()
		}

		for _, s := range resolvable {
			if err = v.Read(s); err != nil {
				return err
			}
		}

		return syscall.Exec(bin, argv, os.Environ())
	})

	return cmd
}

func main() {
	var (
		err error
	)

	app := kingpin.New("vaultenv", "converts vault secrets to environment variables")
	envcmd(app.Command("env", "write secrets to stdout or file")).Default()
	execcmd(app.Command("exec", "execute a command after loading the environment"))

	if _, err = app.Parse(os.Args[1:]); err != nil {
		log.Fatalln(err)
		return
	}
}

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"

	"regexp"

	"gopkg.in/urfave/cli.v1"
)

var envFileLineRegex = regexp.MustCompile("^([A-Za-z][0-9A-Za-z_]*)=(.*)")

func loadEnvironment(data []string, getKeyVal func(item string) (key, val string)) map[string]string {
	items := make(map[string]string)
	for _, item := range data {
		key, val := getKeyVal(item)
		items[key] = val
	}
	return items
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	log.SetOutput(os.Stderr)
	log.SetLevel(log.WarnLevel)

	app := cli.NewApp()
	app.Version = "0.1.1"
	app.Name = "secure-environment"
	app.Before = func(c *cli.Context) error {
		debugOn := c.Bool("debug")

		if debugOn {
			log.SetLevel(log.DebugLevel)
			log.Debug("secure-environment debug logging is on.")
		}

		return nil
	}

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "Set debug logging on",
			EnvVar: "SECURE_ENVIRONMENT_DEBUG",
		},
	}

	flags := []cli.Flag{
		cli.StringFlag{
			Name:   "key",
			Usage:  "Sets the key arn",
			EnvVar: "SECURE_ENVIRONMENT_KEY",
		},
		cli.StringFlag{
			Name:   "url",
			Usage:  "url to the environment file",
			EnvVar: "SECURE_ENVIRONMENT_URL",
		},
		cli.StringFlag{
			Name:   "env-type",
			Value:  "envfile",
			Usage:  "content type of the environment file",
			EnvVar: "SECURE_ENVIRONMENT_TYPE",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:   "export",
			Usage:  "Create a bash compatible export output via stdout",
			Action: exportEnv,
			Flags:  flags,
		},
		{
			Name:   "import",
			Usage:  "Transforms an env file into encrypted env",
			Action: importEnv,
			Flags:  flags,
		},
		{
			Name:   "exec",
			Usage:  "Run a command with decrypted env",
			Action: execEnv,
			Flags:  flags,
		},
	}

	app.Run(os.Args)
}

func importEnv(c *cli.Context) error {
	secureEnvironmentURL := c.String("url")
	secureEnvironmentKey := c.String("key")
	secureEnvironmentType := c.String("env-type")

	if secureEnvironmentURL == "" || secureEnvironmentKey == "" || secureEnvironmentType == "" {
		log.Debug("Missing required environment")
		return fmt.Errorf("Missing required environment variables")
	}

	outputFile, err := os.Create(c.Args().Get(1))

	defer outputFile.Close()

	if err != nil {
		return err
	}

	if file, err := os.Open(c.Args().Get(0)); err == nil {
		defer file.Close()

		fileBytes, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}

		cipher, err := NewCipher()
		if err != nil {
			return nil
		}

		encryptedEnvelope, err := cipher.Encrypt(secureEnvironmentKey, fileBytes)
		if err != nil {
			return err
		}

		return s3PutObject(secureEnvironmentURL, encryptedEnvelope)
	}
	return nil
}

func exportEnv(c *cli.Context) error {
	secureEnvironmentURL := c.String("url")
	secureEnvironmentKey := c.String("key")

	env := os.Environ()
	err := decryptEnv(secureEnvironmentURL, secureEnvironmentKey, &env, true)
	if err != nil {
		return err
	}

	for _, line := range env {
		fmt.Printf("export %s\n", line)
	}

	return nil
}

func execEnv(c *cli.Context) error {
	secureEnvironmentURL := c.String("url")
	secureEnvironmentKey := c.String("key")

	env := os.Environ()
	err := decryptEnv(secureEnvironmentURL, secureEnvironmentKey, &env, false)
	if err != nil {
		return err
	}

	args := c.Args()
	ecmd := exec.Command(args.First(), args.Tail()...)
	ecmd.Stdin = os.Stdin
	ecmd.Stdout = os.Stdout
	ecmd.Stderr = os.Stderr
	ecmd.Env = env

	// Forward SIGINT, SIGTERM, SIGKILL to the child command
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, os.Interrupt, os.Kill)

	go func() {
		sig := <-sigChan
		if ecmd.Process != nil {
			ecmd.Process.Signal(sig)
		}
	}()

	var waitStatus syscall.WaitStatus
	if err := ecmd.Run(); err != nil {
		if err != nil {
			return err
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			os.Exit(waitStatus.ExitStatus())
		}
	}
	return nil
}

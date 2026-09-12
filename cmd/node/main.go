package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"DDNSServer/nodeagent"
)

var version = "dev"

func main() {
	if handled, err := nodeagent.MaybeRunService(version); handled {
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	if len(os.Args) < 2 {
		log.Fatal("usage: DomainSpriteNode <install|run|status|restart|uninstall>")
	}
	command := os.Args[1]
	switch command {
	case "install":
		install()
	case "run":
		run()
	case "status", "restart", "uninstall":
		if err := nodeagent.ServiceAction(command); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("unknown command")
	}
}
func install() {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	name := fs.String("name", "", "node name")
	remark := fs.String("remark", "", "remark")
	controller := fs.String("controller", "", "controller gRPC URL")
	token := fs.String("token", "", "registration token")
	capacity := fs.Int("capacity", 1, "concurrency capacity")
	_ = fs.Parse(os.Args[2:])
	reader := bufio.NewReader(os.Stdin)
	ask := func(label string, target *string) {
		if *target == "" {
			fmt.Print(label + ": ")
			value, _ := reader.ReadString('\n')
			*target = strings.TrimSpace(value)
		}
	}
	ask("Name", name)
	ask("Remark", remark)
	ask("Controller", controller)
	ask("Token", token)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := nodeagent.Register(ctx, *name, *remark, *controller, *token, int32(*capacity))
	if err != nil {
		log.Fatal(err)
	}
	path := nodeagent.DefaultConfigPath()
	if err = nodeagent.SaveConfig(path, cfg); err != nil {
		log.Fatal(err)
	}
	if err = nodeagent.InstallService(path); err != nil {
		log.Fatal(err)
	}
	log.Printf("node %d registered and service installed", cfg.NodeID)
}
func run() {
	cfg, err := nodeagent.LoadConfig(nodeagent.DefaultConfigPath())
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	for {
		err = nodeagent.Run(ctx, cfg, version)
		if ctx.Err() != nil {
			return
		}
		log.Printf("connection lost: %v; reconnecting", err)
		time.Sleep(5 * time.Second)
	}
}

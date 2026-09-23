package main

import (
	"fmt"
	"os"

	"github.com/castrojo/bootc-ecosystem/internal/builds"
	"github.com/castrojo/bootc-ecosystem/internal/quay"
)

func main() {
	// Default to fetch-homebrew for backward compatibility with `just sync`.
	cmd := "fetch-homebrew"
	if len(os.Args) >= 2 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "fetch-homebrew":
		if err := runFetchHomebrew(); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "fetch-testhub":
		if err := runFetchTesthub(); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "fetch-countme":
		if err := runFetchCountme(); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "fetch-contributors":
		if err := runFetchContributors(); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "fetch-releases":
		if err := runFetchReleases(); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "fetch-builds-bluefin":
		if err := runFetchBuildsFor("bluefin", builds.BluefinRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-builds-bluefin:", err)
			os.Exit(1)
		}
	case "fetch-builds-aurora":
		if err := runFetchBuildsFor("aurora", builds.AuroraRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-builds-aurora:", err)
			os.Exit(1)
		}
	case "fetch-builds-bazzite":
		if err := runFetchBuildsFor("bazzite", builds.BazziteRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-builds-bazzite:", err)
			os.Exit(1)
		}
	case "fetch-builds-universal-blue":
		if err := runFetchBuildsFor("universal-blue", builds.UniversalBlueRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-builds-universal-blue:", err)
			os.Exit(1)
		}
	case "fetch-builds-ucore":
		if err := runFetchBuildsFor("ucore", builds.UCoreRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-builds-ucore:", err)
			os.Exit(1)
		}
	case "fetch-builds-zirconium":
		if err := runFetchBuildsFor("zirconium", builds.ZirconiumRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-builds-zirconium:", err)
			os.Exit(1)
		}
	case "fetch-builds-bootcrew":
		if err := runFetchBuildsFor("bootcrew", builds.BootcrewRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-builds-bootcrew:", err)
			os.Exit(1)
		}
	case "fetch-builds-blue-build":
		if err := runFetchBuildsFor("blue-build", builds.BlueBuildRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-builds-blue-build:", err)
			os.Exit(1)
		}
	case "fetch-quay-fedora":
		if err := runFetchQuay("fedora", quay.FedoraRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-quay-fedora:", err)
			os.Exit(1)
		}
	case "fetch-quay-centos":
		if err := runFetchQuay("centos", quay.CentOSRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-quay-centos:", err)
			os.Exit(1)
		}
	case "fetch-quay-almalinux":
		if err := runFetchQuay("almalinux", quay.AlmaLinuxRepos); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-quay-almalinux:", err)
			os.Exit(1)
		}
	case "fetch-scorecard":
		if err := runFetchScorecard(); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-scorecard:", err)
			os.Exit(1)
		}
	case "fetch-supply-chain":
		if err := runFetchSupplyChain(); err != nil {
			fmt.Fprintln(os.Stderr, "❌ fetch-supply-chain:", err)
			os.Exit(1)
		}
	case "fetch-brewfile-taps":
		if err := runFetchBrewfileTaps(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", cmd)
		fmt.Fprintln(os.Stderr, "usage: stats [fetch-homebrew|fetch-testhub|fetch-countme|fetch-contributors|fetch-releases|fetch-builds-bluefin|fetch-builds-aurora|fetch-builds-bazzite|fetch-builds-universal-blue|fetch-builds-ucore|fetch-builds-zirconium|fetch-builds-bootcrew|fetch-builds-blue-build|fetch-quay-fedora|fetch-quay-centos|fetch-quay-almalinux|fetch-scorecard|fetch-supply-chain]")
		os.Exit(1)
	}
}

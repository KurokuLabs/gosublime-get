package main

import (
	"flag"
	"fmt"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	logs = log.New(os.Stderr, "", 0)
)

func main() {
	var subcmd func(*flag.FlagSet, []string)
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "install", "-Syu":
			subcmd = install
		case "uninstall", "-R":
			subcmd = uninstall
		}
	}
	if subcmd == nil {
		logs.Printf("Usage: %s <install|uninstall>", os.Args[0])
		os.Exit(1)
	}
	subcmd(flag.NewFlagSet(os.Args[0], flag.ExitOnError), os.Args[2:])
}

func install(flags *flag.FlagSet, args []string) {
	packages, err := sublimePackagesDir()
	if err != nil {
		logs.Fatalln(err)
	}

	force := false
	version := "latest"
	flags.StringVar(&packages, "packages", packages, "Path where Sublime Text 3 packages are stored.")
	flags.StringVar(&version, "version", version, "The version tag (e.g. v19.12.30) or branch to install.\n  - Enter 'latest' to install the latest stable release branch (development).\n  - Enter 'beta' to install the unreleased development branch (next).")
	flags.BoolVar(&force, "force", force, "Force apply git operations, even in cases where data might be overwritten.")
	flags.Parse(args)

	switch version {
	case "latest":
		version = "development"
	case "beta":
		version = "next"
	}

	versionRef := referenceName(version)
	gsDir := gosublimeDir(packages)
	repoURL := "https://margo.sh/GoSublime"
	remoteName := "gosublime-get"

	repo, err := git.PlainClone(gsDir, false, &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: versionRef,
		RemoteName:    remoteName,
		Progress:      os.Stdout,
	})
	if err == git.ErrRepositoryAlreadyExists {
		repo, err = git.PlainOpen(gsDir)
	}
	if err != nil {
		log.Fatalf("Cannot clone/open %s: %s\n", repoURL, err)
	}

	if _, err := repo.Remote(remoteName); err != nil {
		repo.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{repoURL},
		})
	}

	err = repo.Fetch(&git.FetchOptions{
		Force:      force,
		Progress:   os.Stdout,
		RemoteName: remoteName,
		Tags:       git.AllTags,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		log.Fatalln("git fetch failed:", err)
	}

	tree, err := repo.Worktree()
	if err != nil {
		log.Fatalln("Failed to open git worktree:", err)
	}

	err = tree.Checkout(&git.CheckoutOptions{Branch: versionRef, Force: force})
	if err != nil {
		err := tree.Checkout(&git.CheckoutOptions{
			Branch: versionRef,
			Force:  force,
			Create: true,
		})
		if err != nil {
			log.Fatalln("git checkout failed:", err)
		}
	}

	err = tree.Pull(&git.PullOptions{
		RemoteName:    remoteName,
		ReferenceName: versionRef,
		Progress:      os.Stdout,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		log.Fatalf("git pull(%s) failed: %s\n", versionRef, err)
	}
	fmt.Printf("GoSublime installed in %s.\n", gsDir)
	fmt.Println("You might need to restart Sublime Text for changes to take effect.", gsDir)
}

func sublimePackagesDir() (string, error) {
	dir := map[string]string{
		"linux":   "$HOME/.config/sublime-text-3/Packages",
		"darwin":  "$HOME/Library/Application Support/Sublime Text 3/Packages",
		"windows": "$APPDATA\\Sublime Text 3\\Packages",
	}[runtime.GOOS]
	dir = os.ExpandEnv(dir)
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("Packages dir `%s` is not absolute\n", dir)
	}
	if _, err := os.Lstat(dir); err != nil {
		return "", fmt.Errorf("Cannot stat packages dir `%s`: %s\n", dir, err)
	}
	return dir, nil
}

func referenceName(name string) plumbing.ReferenceName {
	if strings.Contains(name, ".") {
		return plumbing.NewTagReferenceName(name)
	}
	return plumbing.NewBranchReferenceName(name)
}

func uninstall(flags *flag.FlagSet, args []string) {
	packages, err := sublimePackagesDir()
	if err != nil {
		logs.Fatalln(err)
	}
	flags.StringVar(&packages, "packages", packages, "Path where Sublime Text 3 packages are stored.")
	flags.Parse(args)

	gsDir := gosublimeDir(packages)
	fi, err := os.Stat(gsDir)
	if err != nil {
		if os.IsNotExist(err) {
			logs.Fatalf("GoSublime not installed: stat(%s): %s\n", gsDir, err)
		}
		logs.Fatalf("GoSublime not installed in %s\n", gsDir, err)
	}
	if !fi.IsDir() {
		logs.Fatalf("%s is not a directory: %s\n", gsDir, err)
	}
	confirm := ""
	fmt.Printf("Are you sure you want to remove directory %s?\nEnter 'Y' or 'y' to confirm: ", gsDir)
	fmt.Scanf("%s", &confirm)
	if confirm != "Y" && confirm != "y" {
		return
	}
	if err := os.RemoveAll(gsDir); err != nil {
		logs.Fatalf("Cannot remove %s: %s\n", gsDir, err)
	}
	fmt.Println(gsDir, "removed")
}

func gosublimeDir(packagesDir string) string {
	return filepath.Join(packagesDir, "GoSublime")
}

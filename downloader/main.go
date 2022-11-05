package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

const (
	OBSIDIAN_PLUGINS_GITHUB_PATH = "obsidianmd/obsidian-releases"
	PLUGINS_JSON_FILENAME        = "community-plugins.json"
)

var PLUGIN_RELEASE_FILES = [...]string{"manifest.json", "styles.css", "main.js"}

type Plugin struct {
	Repo string `json:"repo"`
}

func updateRepo(repoFolder string, repoUrlPath string) error {
	_, err := git.PlainClone(
		repoFolder,
		false,
		&git.CloneOptions{URL: fmt.Sprintf("https://github.com/%s", repoUrlPath)},
	)
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		return err
	}

	if err == git.ErrRepositoryAlreadyExists {
		repo, err := git.PlainOpen(repoFolder)
		if err != nil {
			return fmt.Errorf("[!] Error opening repo: %s, %s", repoUrlPath, err)
		}

		worktree, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("[!] Error getting worktree: %s, %s", repoUrlPath, err)
		}
		if err := worktree.Pull(&git.PullOptions{}); err != nil && err != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("[!] Error pulling changes: %s, %s", repoUrlPath, err)
		}
	}
	return nil
}

func downloadLatestPluginRelease(pluginFolder string, pluginUrlPath string) error {
	file, err := os.Open(filepath.Join(pluginFolder, "manifest.json"))
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	manifest := struct {
		Version string `json:"version"`
	}{}
	if err = decoder.Decode(&manifest); err != nil {
		return err
	}

	var releaseFolder = filepath.Join(pluginFolder, "releases", "download", manifest.Version)
	if err := os.MkdirAll(releaseFolder, os.ModeDir); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, releaseFile := range PLUGIN_RELEASE_FILES {
		wg.Add(1)
		go func(releaseFile string) {
			defer wg.Done()
			filePath := filepath.Join(releaseFolder, releaseFile)
			if _, err := os.Stat(filePath); err == nil || os.IsExist(err) {
				return
			}

			out, err := os.Create(filePath)
			if err != nil {
				log.Printf("%v\n\n", err)
				return
			}
			defer out.Close()

			resp, err := http.Get(fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", pluginUrlPath, manifest.Version, releaseFile))
			if err != nil {
				log.Printf("%v\n\n", err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				_, err = io.Copy(out, resp.Body)
				if err != nil {
					log.Printf("%v\n\n", err)
					return
				}
			}
		}(releaseFile)
	}
	wg.Wait()
	return nil
}

func updatePlugin(pluginFolder string, pluginUrlPath string) error {
	if err := updateRepo(pluginFolder, pluginUrlPath); err != nil {
		return err
	}
	if err := downloadLatestPluginRelease(pluginFolder, pluginUrlPath); err != nil {
		return fmt.Errorf("[!] Error downloading latest release: %s, %s", pluginUrlPath, err)
	}
	return nil
}

func getPlugins(pluginsPath string) ([]Plugin, error) {
	file, err := os.Open(filepath.Join(pluginsPath, OBSIDIAN_PLUGINS_GITHUB_PATH, PLUGINS_JSON_FILENAME))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	plugins := []Plugin{}
	err = decoder.Decode(&plugins)
	if err != nil {
		return nil, err
	}
	return plugins, nil
}

func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func downloadPlugins(pluginsPath string, plugins []Plugin) {
	var wg sync.WaitGroup
	size := int64(0)
	pool := make(chan struct{}, 20)
	bar := mpb.New(mpb.WithWidth(80)).AddBar(
		int64(len(plugins)),
		mpb.PrependDecorators(decor.Percentage()),
		mpb.AppendDecorators(
			decor.CountersNoUnit("(%d/%d "),
			decor.Elapsed(decor.ET_STYLE_MMSS),
			decor.Name(":"),
			decor.EwmaETA(decor.ET_STYLE_MMSS, 60),
			decor.Any(func(_ decor.Statistics) string {
				return fmt.Sprintf(" %.2v", decor.SizeB1024(size))
			}),
			decor.Name(")"),
		),
	)
	for _, plugin := range plugins {
		wg.Add(1)
		pool <- struct{}{}
		go func(pluginRepo string) {
			defer func() {
				<-pool
				wg.Done()
			}()

			start := time.Now()
			pluginPath := filepath.Join(pluginsPath, pluginRepo)
			done := make(chan struct{})
			go func() {
				if err := updatePlugin(pluginPath, pluginRepo); err != nil {
					log.Printf("%v\n\n", err)
				}
				close(done)
			}()

			<-done
			currentSize, err := dirSize(pluginPath)
			if err == nil {
				atomic.AddInt64(&size, currentSize)
			}
			bar.EwmaIncrement(time.Since(start))
		}(plugin.Repo)
	}
	close(pool)
	wg.Wait()
}

func main() {
	log.Println("[*] Getting obsidian repo.")
	var pluginsPath = filepath.Join(".", "plugins")
	if err := os.MkdirAll(pluginsPath, os.ModeDir); err != nil {
		log.Fatal(err)
	}
	if err := updateRepo(filepath.Join(pluginsPath, OBSIDIAN_PLUGINS_GITHUB_PATH), OBSIDIAN_PLUGINS_GITHUB_PATH); err != nil {
		log.Fatal(err)
	}

	log.Println("[*] Getting plugin list.")
	plugins, err := getPlugins(pluginsPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("[*] Downloading plugins.")
	downloadPlugins(pluginsPath, plugins)
}

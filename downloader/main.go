package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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

var (
	MINIMAL              bool
	PLUGIN_RELEASE_FILES = [...]string{"manifest.json", "styles.css", "main.js"}
	PLUGIN_MINIMAL_FILES = [...]string{"manifest.json", "README.md"}
)

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

func downloadFileIfChanged(fileUrl string, filePath string) {
	var out *os.File
	var outInfo os.FileInfo
	var err error
	if outInfo, err = os.Stat(filePath); os.IsNotExist(err) {
		if out, err = os.Create(filePath); err == nil {
			outInfo, _ = out.Stat()
		}
	} else {
		out, err = os.OpenFile(filePath, os.O_RDWR, os.ModeType)
	}
	if err != nil {
		log.Printf("%v\n\n", err)
		return
	}
	defer out.Close()

	resp, err := http.Get(fileUrl)
	if err != nil {
		log.Printf("%v\n\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("%v\n\n", err)
		return
	}

	if resp.StatusCode == 200 && outInfo.Size() != int64(len(body)) {
		_, err = out.Write(body)
		if err != nil {
			log.Printf("%v\n\n", err)
			return
		}
	}
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
			downloadFileIfChanged(
				fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", pluginUrlPath, manifest.Version, releaseFile),
				filepath.Join(releaseFolder, releaseFile),
			)
		}(releaseFile)
	}
	wg.Wait()
	return nil
}

func updatePlugin(pluginFolder string, pluginUrlPath string) error {
	if err := os.MkdirAll(pluginFolder, os.ModeDir); err != nil {
		return err
	}

	if MINIMAL {
		for _, file := range PLUGIN_MINIMAL_FILES {
			downloadFileIfChanged(
				fmt.Sprintf("https://raw.githubusercontent.com/%s/HEAD/%s", pluginUrlPath, file),
				filepath.Join(pluginFolder, file),
			)
		}
	} else {
		if err := updateRepo(pluginFolder, pluginUrlPath); err != nil {
			return err
		}
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
	flag.BoolVar(&MINIMAL, "minimal", false, "Download only README.md, manifest.json + release files for plugins.")
	flag.Parse()

	log.Println("[*] Getting obsidian repo.")
	var pluginsPath = filepath.Join(".", "plugins")
	if MINIMAL {
		pluginsPath = filepath.Join(".", "plugins-minimal")
	}

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

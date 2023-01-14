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
	"github.com/samber/lo"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

const (
	OBSIDIAN_GITHUB_PATH  = "obsidianmd/obsidian-releases"
	PLUGINS_JSON_FILENAME = "community-plugins.json"
	THEMES_JSON_FILENAME  = "community-css-themes.json"
	DESKTOP_RELEASES_FILE = "desktop-releases.json"
)

var (
	PLUGIN_RELEASE_FILES = []string{"manifest.json", "styles.css", "main.js"}
	PLUGIN_FILES         = []string{"manifest.json", "README.md"}
	THEMES_FILES         = []string{"manifest.json", "README.md", "theme.css", "obsidian.css"}
)

type Repo struct {
	Repo       string
	isTheme    bool
	isPlugin   bool
	extraFiles []string
}

func updateLocalGitRepo(repoFolder string, repoUrlPath string) error {
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

func getOrCreateFile(filePath string) (*os.File, os.FileInfo, error) {
	var err error
	fileDir := filepath.Dir(filePath)
	if _, err = os.Stat(fileDir); os.IsNotExist(err) {
		if err = os.MkdirAll(fileDir, os.ModeDir); err != nil {
			return nil, nil, err
		}
	}
	if err != nil {
		return nil, nil, err
	}

	var out *os.File
	var outInfo os.FileInfo
	if outInfo, err = os.Stat(filePath); os.IsNotExist(err) {
		if out, err = os.Create(filePath); err == nil {
			outInfo, _ = out.Stat()
		}
	} else {
		out, err = os.OpenFile(filePath, os.O_RDWR, os.ModeType)
	}
	if err != nil {
		return nil, nil, err
	}
	return out, outInfo, nil
}

func downloadFileIfChanged(fileUrl string, filePath string) {
	var err error
	resp, err := http.Get(fileUrl)
	if err != nil {
		log.Printf("%v\n\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("%v\n\n", err)
		return
	}

	bodySize := int64(len(body))
	if resp.StatusCode != 200 || bodySize == 0 {
		return
	}

	out, outInfo, err := getOrCreateFile(filePath)
	if err != nil {
		log.Printf("%v\n\n", err)
		return
	}
	defer out.Close()
	if outInfo.Size() == bodySize {
		return
	}

	_, err = out.Write(body)
	if err != nil {
		log.Printf("%v\n\n", err)
		return
	}
}

func downloadLatestPluginRelease(pluginFolder string, pluginUrlPath string) error {
	resp, err := http.Get(fmt.Sprintf("https://github.com/%s/releases", pluginUrlPath))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}

	file, err := os.Open(filepath.Join(pluginFolder, "manifest.json"))
	if err != nil {
		return err
	}
	defer file.Close()

	manifest := struct {
		Version string
	}{}
	if err = json.NewDecoder(file).Decode(&manifest); err != nil {
		return err
	}

	var releaseFolder = filepath.Join(pluginFolder, "releases", "download", manifest.Version)
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

func downloadFilesFromGithub(repo Repo, folder string, files []string) {
	for _, file := range append(files, repo.extraFiles...) {
		downloadFileIfChanged(
			fmt.Sprintf("https://raw.githubusercontent.com/%s/HEAD/%s", repo.Repo, file),
			filepath.Join(folder, file),
		)
	}
}

func updateRepo(repoFolder string, repo Repo) error {
	if repo.isPlugin {
		downloadFilesFromGithub(repo, repoFolder, PLUGIN_FILES)
		if err := downloadLatestPluginRelease(repoFolder, repo.Repo); err != nil {
			return fmt.Errorf("[!] Error downloading latest release: %s, %s", repo.Repo, err)
		}
	} else if repo.isTheme {
		downloadFilesFromGithub(repo, repoFolder, THEMES_FILES)
	} else {
		return fmt.Errorf("[!] Repo: %s is not a plugin nor a theme", repo.Repo)
	}

	return nil
}

func getPluginsAndThemesRepos(downloadFolder string) []*Repo {
	var repos = make(map[string]*Repo)

	pluginsFile, _ := os.Open(filepath.Join(downloadFolder, OBSIDIAN_GITHUB_PATH, PLUGINS_JSON_FILENAME))
	defer pluginsFile.Close()
	themesFile, _ := os.Open(filepath.Join(downloadFolder, OBSIDIAN_GITHUB_PATH, THEMES_JSON_FILENAME))
	defer themesFile.Close()

	plugins := []struct {
		Repo string
	}{}
	themes := []struct {
		Repo       string
		Screenshot string
	}{}
	json.NewDecoder(pluginsFile).Decode(&plugins)
	json.NewDecoder(themesFile).Decode(&themes)
	for i := range plugins {
		repo := plugins[i].Repo
		repos[repo] = &Repo{
			Repo:     repo,
			isPlugin: true,
		}
	}

	for i := range themes {
		theme := themes[i]
		if repo, ok := repos[theme.Repo]; ok {
			repo.isTheme = true
			repo.extraFiles = append(repo.extraFiles, theme.Screenshot)
		} else {
			repos[theme.Repo] = &Repo{
				Repo:       theme.Repo,
				isTheme:    true,
				extraFiles: []string{theme.Screenshot},
			}
		}
	}
	return lo.Values(repos)
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

func downloadPluginsAndThemes(downloadFolder string, pluginsAndThemesRepos []*Repo) {
	var wg sync.WaitGroup
	size := int64(0)
	pool := make(chan struct{}, 20)
	bar := mpb.New(mpb.WithWidth(80)).AddBar(
		int64(len(pluginsAndThemesRepos)),
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

	for _, repo := range pluginsAndThemesRepos {
		wg.Add(1)
		pool <- struct{}{}
		go func(repo Repo) {
			defer func() {
				<-pool
				wg.Done()
			}()

			start := time.Now()
			repoFolder := filepath.Join(downloadFolder, repo.Repo)
			done := make(chan struct{})
			go func() {
				if err := updateRepo(repoFolder, repo); err != nil {
					log.Printf("%v\n\n", err)
				}
				close(done)
			}()

			<-done
			currentSize, err := dirSize(repoFolder)
			if err == nil {
				atomic.AddInt64(&size, currentSize)
			}
			bar.EwmaIncrement(time.Since(start))
		}(*repo)
	}
	close(pool)
	wg.Wait()
}

func downloadThemesStats(downloadFolder string) {
	downloadFileIfChanged("https://releases.obsidian.md/stats/theme", filepath.Join(downloadFolder, "stats", "theme"))
}

func downloadLatestDesktopRelease(downloadFolder string) {
	releasesFile, _ := os.Open(filepath.Join(downloadFolder, OBSIDIAN_GITHUB_PATH, DESKTOP_RELEASES_FILE))
	defer releasesFile.Close()
	releases := struct {
		LatestVersion string
		DownloadUrl   string
	}{}
	json.NewDecoder(releasesFile).Decode(&releases)

	latestReleasePath := fmt.Sprintf("%s/releases/download/v%s/obsidian-%s.asar.gz", OBSIDIAN_GITHUB_PATH, releases.LatestVersion, releases.LatestVersion)
	downloadFileIfChanged(fmt.Sprintf("https://github.com/%s", latestReleasePath), filepath.Join(downloadFolder, latestReleasePath))
}

func main() {
	log.Println("[*] Pulling obsidian repo.")
	var downloadFolder = filepath.Join(".", "files")
	obsidianReleasesFolder := filepath.Join(downloadFolder, OBSIDIAN_GITHUB_PATH)
	if err := os.MkdirAll(downloadFolder, os.ModeDir); err != nil {
		log.Fatal(err)
	}
	if err := os.RemoveAll(obsidianReleasesFolder); err != nil {
		log.Fatal(err)
	}
	if err := updateLocalGitRepo(obsidianReleasesFolder, OBSIDIAN_GITHUB_PATH); err != nil {
		log.Fatal(err)
	}

	log.Println("[*] Downloading latest desktop release, don't forget to patch it later!")
	downloadLatestDesktopRelease(downloadFolder)

	log.Println("[*] Downloading themes stats")
	downloadThemesStats(downloadFolder)

	log.Println("[*] Getting repos list.")
	pluginsAndThemesRepos := getPluginsAndThemesRepos(downloadFolder)

	fmt.Println("[*] Downloading repos.")
	downloadPluginsAndThemes(downloadFolder, pluginsAndThemesRepos)
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/joho/godotenv"
	"github.com/ktrysmt/go-bitbucket"
	"github.com/sirupsen/logrus"
)

type BitbucketConfig struct {
	Username     string
	Password     string
	Owner        string
	RepoSlug     string
	CommitHash   string
	ChangeLogTag string
}

var cfg BitbucketConfig

func init() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("failed to load from .env file, falling back to environment variables.")
	}

	cfg = BitbucketConfig{
		Username:   os.Getenv("BITBUCKET_USERNAME"),
		Password:   os.Getenv("BITBUCKET_PASSWORD"),
		Owner:      os.Getenv("BITBUCKET_WORKSPACE"),
		RepoSlug:   os.Getenv("BITBUCKET_REPO_SLUG"),
		CommitHash: os.Getenv("BITBUCKET_COMMIT"),
	}
	validateConfig(cfg)
}

func validateConfig(cfg BitbucketConfig) {
	if cfg.Username == "" {
		panic("BITBUCKET_USERNAME is not set in the environment")
	}
	if cfg.Password == "" {
		panic("BITBUCKET_PASSWORD is not set in the environment")
	}
	if cfg.Owner == "" {
		panic("BITBUCKET_WORKSPACE is not set in the environment")
	}
	if cfg.RepoSlug == "" {
		panic("BITBUCKET_REPO_SLUG is not set in the environment")
	}
	if cfg.CommitHash == "" {
		panic("BITBUCKET_COMMIT is not set in the environment")
	}
}

func GetLatestTagFromChangelog() (string, error) {
	cmdName := "changelogmanager"
	cmdArgs := []string{"version"}
	cmd := exec.Command(cmdName, cmdArgs...)
	cmdOutput, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("couldn't get the last tag from changelogmanager: %w", err)
	}

	reg, err := regexp.Compile("[^a-zA-Z0-9.]+")
	if err != nil {
		return "", fmt.Errorf("failed to compile regex: %w", err)
	}

	output := reg.ReplaceAllString(string(cmdOutput), "")
	return fmt.Sprintf("v%s", output), nil
}

func main() {
	// Setup logger
	var logger = logrus.New()
	logger.WithFields(logrus.Fields{
		"Repository Name": cfg.RepoSlug,
		"Workspace":       cfg.Owner,
		"Commit Hash":     cfg.CommitHash,
	}).Info("initilizing release-manager pipe...")

	// Get latest tag
	// add that tag to the config
	latestTag, err := GetLatestTagFromChangelog()
	if err != nil {
		logger.WithError(err).Fatal("failed to get the latest tag from changelog")
	}

	cfg.ChangeLogTag = latestTag

	logger.WithFields(logrus.Fields{
		"Changelog Tag": cfg.ChangeLogTag,
	}).Info("retrieved the latest tag from changelog")

	// Initialize bitbucket client
	logger.Debugf("Initializing bitbucket client with user %s", cfg.Username)
	client := bitbucket.NewBasicAuth(cfg.Username, cfg.Password)

	// Check if the release tag already exists
	bitbucket := Bitbucket{
		Client: *client,
		RepositoryTagOptions: bitbucket.RepositoryTagOptions{
			Owner:    cfg.Owner,
			RepoSlug: cfg.RepoSlug,
		},
		BitbucketConfig: cfg,
	}
	repoTags, err := bitbucket.GetAllTagsAndCommitHashes()
	if err != nil {
		panic("couldn't get the list of tags")
	}

	if hasTag(repoTags, cfg.ChangeLogTag) {
		logger.Infof("changelog tag %s already exists in the repo...", cfg.ChangeLogTag)
		return
	}

	// Tag with ne release tag
	logger.Info("tagging commit...")
	_, err = bitbucket.tagCommit()
	if err != nil {
		logger.Fatalf("failed to tag commit %s with %s", cfg.CommitHash, cfg.ChangeLogTag)
	}
	logger.Infof("commit %s was tagged with %s", cfg.CommitHash, cfg.ChangeLogTag)
}

func hasTag(tagList []Tag, tag string) bool {
	for _, t := range tagList {
		if t.Name == tag {
			return true
		}
	}
	return false
}

type Bitbucket struct {
	Client               bitbucket.Client
	RepositoryTagOptions bitbucket.RepositoryTagOptions
	BitbucketConfig      BitbucketConfig
}

func (b Bitbucket) tagCommit() (*bitbucket.RepositoryTag, error) {
	return b.Client.Repositories.Repository.CreateTag(
		&bitbucket.RepositoryTagCreationOptions{
			Owner:    b.RepositoryTagOptions.Owner,
			RepoSlug: b.RepositoryTagOptions.RepoSlug,
			Name:     b.BitbucketConfig.ChangeLogTag,
			Target: bitbucket.RepositoryTagTarget{
				Hash: b.BitbucketConfig.CommitHash,
			},
		},
	)
}

func (b Bitbucket) GetAllTagsAndCommitHashes() ([]Tag, error) {
	var page int = 1
	var pageLen int = 100

	var tagList []Tag
	for {
		tags, err := b.Client.Repositories.Repository.ListTags(&bitbucket.RepositoryTagOptions{
			Owner:    cfg.Owner,
			RepoSlug: cfg.RepoSlug,
			PageNum:  page,
			Pagelen:  pageLen,
		})
		if err != nil {
			return nil, fmt.Errorf("couldn't get list of tags from the repo: %w", err)
		}

		tagList = append(tagList, extractTag(tags)...)

		if tags.Next == "" {
			break
		} else {
			page++
		}
	}

	return tagList, nil
}

type Tag struct {
	Name string
}

func extractTag(tags *bitbucket.RepositoryTags) []Tag {
	var tagList []Tag
	for _, tag := range tags.Tags {
		tagList = append(tagList, Tag{
			Name: tag.Name,
		})
	}
	return tagList
}

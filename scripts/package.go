package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"log"
	"io/ioutil"
	"strings"
	"net/http"
	"time"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/layer5io/meshkit/utils/catalog"
	"github.com/layer5io/meshkit/models/catalog/v1alpha1"
)

type CatalogPattern struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	PatternFile  string `json:"pattern_file"`
	CatalogData struct {
        PatternInfo   string        `json:"pattern_info"`
        PatternCaveats string      `json:"pattern_caveats"`
        Type          string        `json:"type"`
        ImageURL      interface{}   `json:"imageURL"`
        Compatibility []string      `json:"compatibility"`
    } `json:"catalog_data"`
	UserID string `json:"user_id"`
}

type UserInfo struct {
	UserID    string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
}

const (
	mesheryCloudBaseURL    = "https://meshery.layer5.io"
	mesheryCatalogFilesDir = "catalog"
)

func main() {
	//token := os.Getenv("GH_ACCESS_TOKEN")
	catalogPatterns := fetchCatalogPatterns()
	if catalogPatterns == nil {
		return
	}

	var patterns []CatalogPattern
	if err := json.Unmarshal(catalogPatterns, &patterns); err != nil {
		log.Fatalf("Error parsing catalog patterns: %v", err)
	}

	for _, pattern := range patterns {
		processPattern(pattern)
	}
}

func fetchCatalogPatterns() []byte {
	resp, err := http.Get(fmt.Sprintf("%s/api/catalog/content/pattern", mesheryCloudBaseURL))
	if err != nil {
		log.Printf("Error connecting to Meshery Cloud: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v\n", err)
		return nil
	}
	return body
}

func processPattern(pattern CatalogPattern) {
	patternImageURL := getPatternImageURL(pattern)
	patternType := getPatternType(pattern.CatalogData.Type)
	patternInfo := getStringOrEmpty(pattern.CatalogData.PatternInfo)
	patternCaveats := getStringOrEmpty(pattern.CatalogData.PatternCaveats)

	compatibility := getCompatibility(pattern.CatalogData.Compatibility)

	dir := fmt.Sprintf("./collections/_catalog/%s", patternType)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Printf("%s doesn't exist. Creating directory...\n", patternType)
		os.MkdirAll(dir, 0755)
	}

	if err := writePatternFile(pattern, patternType, patternInfo, patternCaveats, compatibility, patternImageURL); err != nil {
		log.Printf("Fialed to write pattern file")
	}
}

func getPatternImageURL(pattern CatalogPattern) string {
	var imageURL string
	if pattern.CatalogData.ImageURL == nil {
		imageURL = fmt.Sprintf("https://raw.githubusercontent.com/layer5labs/meshery-extensions-packages/master/action-assets/design-assets/%s-light.png,https://raw.githubusercontent.com/layer5labs/meshery-extensions-packages/master/action-assets/design-assets/%s-dark.png", pattern.ID, pattern.ID)
	} else {
		switch v := pattern.CatalogData.ImageURL.(type) {
		case string:
			imageURL = v
		case []interface{}:
			var urls []string
			for _, u := range v {
				urls = append(urls, u.(string))
			}
			imageURL = strings.Join(urls, ",")
		}
	}
	return imageURL
}

func getPatternType(patternType string) string {
	if patternType == "" {
		patternType = "Deployment"
	}
	return strings.ToLower(strings.ReplaceAll(patternType, " ", "-"))
}

func getStringOrEmpty(value string) string {
	if value == "" {
		return "\"\""
	}
	return value
}

func getCompatibility(compatibility []string) string {
	var compatLines []string
	for _, compat := range compatibility {
		compatLines = append(compatLines, fmt.Sprintf("    - %s", compat))
	}
	return strings.Join(compatLines, "\n")
}

func writePatternFile(pattern CatalogPattern, patternType, patternInfo, patternCaveats, compatibility, patternImageURL string) error {
	dir := filepath.Join(mesheryCatalogFilesDir, pattern.ID)
	os.MkdirAll(dir, 0755)
	ioutil.WriteFile(filepath.Join(dir, "deploy.yml"), []byte(pattern.PatternFile), 0644)

	services, _ := exec.Command("yq", ".services", filepath.Join(dir, "deploy.yml")).Output()
	if strings.TrimSpace(string(services)) == "" {
		patternImageURL = "/assets/images/logos/service-mesh-pattern.svg"
	}

	//process for versioning is needed 
	format := "2006-01-02 15:04:05Z"
	currentDateTime, err := time.Parse(format, time.Now().UTC().Format(format))

	var snapshotURL []string
    switch v := pattern.CatalogData.ImageURL.(type) {
    case string:
        snapshotURL = []string{v}
    case []string:
        snapshotURL = v
    default:
        snapshotURL = []string{}
    }

    convertedCompatibility := make([]v1alpha1.CatalogDataCompatibility, len(pattern.CatalogData.Compatibility))
    for i, compat := range pattern.CatalogData.Compatibility {
        convertedCompatibility[i] = v1alpha1.CatalogDataCompatibility(compat)
    }

    convertedCatalogData := &v1alpha1.CatalogData{
        PatternInfo:    pattern.CatalogData.PatternInfo,
        PatternCaveats: pattern.CatalogData.PatternCaveats,
        Type:           v1alpha1.CatalogDataType(pattern.CatalogData.Type),
        SnapshotURL:    snapshotURL,
        Compatibility:  convertedCompatibility,
    }

	artifactHubPkg := catalog.BuildArtifactHubPkg(pattern.Name, filepath.Join(dir, "deploy.yml"), pattern.UserID, "1.0.0", currentDateTime.Format(time.RFC3339), convertedCatalogData)
	data, err := yaml.Marshal(artifactHubPkg)
	if err != nil {
		return fmt.Errorf("failed to marshal YML: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "artifacthub-pkg.yml"), data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	userInfo := fetchUserInfo(pattern.UserID)
	userFullName := fmt.Sprintf("%s %s", userInfo.FirstName, userInfo.LastName)

	content := fmt.Sprintf(`---
layout: item
name: %s
userId: %s
userName: %s
userAvatarURL: %s
type: %s
compatibility: %s
patternId: %s
image: %s
patternInfo: |
%s
patternCaveats: |
%s
URL: 'https://raw.githubusercontent.com/meshery/meshery.io/master/%s/%s/deploy.yml'
downloadLink: %s/deploy.yml
---`, pattern.Name, pattern.UserID, userFullName, userInfo.AvatarURL, patternType, compatibility, pattern.ID, patternImageURL, patternInfo, patternCaveats, mesheryCatalogFilesDir, pattern.ID, pattern.ID)

	ioutil.WriteFile(fmt.Sprintf("./collections/_catalog/%s/%s.md", patternType, pattern.ID), []byte(content), 0644)
	
	return nil
}

func fetchUserInfo(userID string) UserInfo {
	resp, err := http.Get(fmt.Sprintf("%s/api/identity/users/profile/%s", mesheryCloudBaseURL, userID))
	if err != nil {
		log.Printf("Error fetching User details: %v\n", err)
		return UserInfo{}
	}
	defer resp.Body.Close()

	var userInfo UserInfo
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading user info response bod: %v\n", err)
		return UserInfo{}
	}
	json.Unmarshal(body, &userInfo)
	return userInfo
}
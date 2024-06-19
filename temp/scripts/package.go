package main

import (
	"encoding/json"
	"fmt"
	"os"
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
	token := os.Getenv("GH_ACCESS_TOKEN")
	catalogPatterns := fetchCatalogPatterns()

	var patterns []CatalogPattern
	if err := json.Unmarshal(catalogPatterns, &patterns); err != nil {
		log.Fatalf("Error parsing catalog patterns: %v", err)
	}

	for _, pattern := range patterns {
		processPattern(pattern, token)
		fmt.Println(pattern.ID)
	}
}

func fetchCatalogPatterns() []byte {
	//resp, err := http.Get(fmt.Sprintf("%s/api/catalog/content/pattern", mesheryCloudBaseURL))
	//if err != nil {
	//	log.Printf("Error connecting to Meshery Cloud: %v\n", err)
	//	return nil
	//}
	//defer resp.Body.Close()

	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	log.Printf("Error reading response body: %v\n", err)
	//	return nil
	//}
	//return body

	tempData := `
	[
		{
			"id": "0194ff83-0a43-4b83-9c75-b8f0f32da6b7",
			"name": "Pod Readiness",
			"pattern_file": "name: Pod Readiness\nservices:\n  pods-readiness-exec-pod:\n    name: pods-readiness-exec-pod\n    type: Pod\n    apiVersion: v1\n    namespace: default\n    model: kubernetes\n    settings:\n      spec:\n        containers:\n          - args:\n              - /bin/sh\n              - -c\n              - touch /tmp/healthy; sleep 30; rm -rf /tmp/healthy; sleep 600\n            image: busybox\n            name: pods-readiness-exec-container\n            readinessProbe:\n              exec:\n                command:\n                  - cat\n                  - /tmp/healthy\n            initialDelaySeconds: 5\n    traits:\n      meshmap:\n        edges: []\n    id: 583718ce-da5a-444e-ab2c-0dd0ee79ecdc\n    meshmodel-metadata:\n      capabilities: \"\"\n      genealogy: \"\"\n      isAnnotation: false\n      isCustomResource: false\n      isModelAnnotation: \"FALSE\"\n      isNamespaced: true\n      logoURL: https://github.com/cncf/artwork/blob/master/projects/kubernetes/icon/white/kubernetes-icon-white.svg\n      model: kubernetes\n      modelDisplayName: Kubernetes\n      primaryColor: '#326CE5'\n      published: true\n      secondaryColor: '#7aa1f0'\n      shape: round-rectangle\n      styleOverrides: \"\"\n      subCategory: Scheduling & Orchestration\n      svgColor: ui/public/static/img/meshmodels/kubernetes/color/kubernetes-color.svg\n      svgComplete: \"\"\n      svgWhite: ui/public/static/img/meshmodels/kubernetes/white/kubernetes-white.svg\n    position:\n      posX: 170\n      posY: 170\n    whiteboardData:\n      style: {}",
			"catalog_data": {
				"pattern_info": "Info about pattern one",
				"pattern_caveats": "Caveats for pattern one",
				"type": "deployment",
				"imageURL": "https://example.com/image1.png",
				"compatibility": ["compat1"]
			},
			"user_id": "99b3cfd0-4269-4ee8-9ac7-392dd24c1c02"
		}
	]`

	return []byte(tempData)
}

func processPattern(pattern CatalogPattern, token string) {
	patternImageURL := getPatternImageURL(pattern)
	patternType := getPatternType(pattern.CatalogData.Type)
	patternInfo := getStringOrEmpty(pattern.CatalogData.PatternInfo)
	patternCaveats := getStringOrEmpty(pattern.CatalogData.PatternCaveats)

	compatibility := getCompatibility(pattern.CatalogData.Compatibility)

	dir := filepath.Join("..", "..", "collections", "_catalog", patternType)
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
	dir := filepath.Join("..", "..", mesheryCatalogFilesDir, pattern.ID)
	deployFilePath := filepath.Join(dir, "deploy.yml")
	os.MkdirAll(dir, 0755)
	ioutil.WriteFile(deployFilePath, []byte(pattern.PatternFile), 0644)

	contenttemp, err := ioutil.ReadFile(deployFilePath)
	if err != nil {
		return fmt.Errorf("Failed to read file: %v\n", err)
	}

	var datatemp map[string]interface{}
	if err := yaml.Unmarshal(contenttemp, &datatemp); err != nil {
		return fmt.Errorf("Failed to unmarshal YAML: %v\n", err)
	}

	if services, ok := datatemp["services"]; !ok || services == nil {
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
compatibility: 
  %s
patternId: %s
image: %s
patternInfo: |
  %s
patternCaveats: |
  %s
URL: 'https://raw.githubusercontent.com/meshery/meshery.io/master/%s/%s/deploy.yml'
downloadLink: %s/deploy.yml
---`, pattern.Name, pattern.UserID, userFullName, userInfo.AvatarURL, patternType, compatibility, pattern.ID, patternImageURL, patternInfo, patternCaveats, mesheryCatalogFilesDir, pattern.ID, pattern.ID)

	ioutil.WriteFile(fmt.Sprintf(filepath.Join("..", "..", "collections", "_catalog", patternType, pattern.ID+".md")), []byte(content), 0644)
	
	return nil
}

func fetchUserInfo(userID string) UserInfo {
	resp, err := http.Get(fmt.Sprintf("%s/api/identity/users/profile/%s", mesheryCloudBaseURL, "99b3cfd0-4269-4ee8-9ac7-392dd24c1c02"))
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
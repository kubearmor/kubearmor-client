package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "strings"
//	"io/ioutil"
	"gopkg.in/yaml.v3"
)

type Package struct {
    Name        string `json:"name"`
    DisplayName string `json:"display_name"`
    Description string `json:"description"`
    Repository  struct {
        Name    string `json:"name"`
        URL     string `json:"url"`
        OrgName string `json:"organization_name"`
    } `json:"repository"`
}

type SearchResponse struct {
    Packages []Package `json:"packages"`
}

type GitHubFile struct {
    Name        string `json:"name"`
    Path        string `json:"path"`
    Type        string `json:"type"` // "file" or "dir"
    DownloadURL string `json:"download_url"`
}

func useHelp() {
    fmt.Println("Use --help or -h to see all help options")
}

// help function to display usage information
func printHelp() {
    fmt.Println(`karmor tool

 Usage:
    karmor recommend --image <image-name> [--limit <number>] [--download] [ --overwrite-labels <key1=value1,key2=value2> | --append-labels <key1=value1,key2=value2> ]

 Flags:
    --image              Image name (e.g., nginx)
    --limit              Number of results to fetch (default: 1)
    --download           If passed, download the policy files (default: false)
    --overwrite-labels   Overwrite spec.selector.matchLabels in the policy YAML with comma-separated labels (e.g. team=dev,env=prod)
    --append-labels      Append into spec.selector.matchLabels in the policy YAML with comma-separated labels (e.g. owner=security,env=poc)
    --help, -h           To display all help options

 Examples:
    karmor recommend --image nginx
    karmor recommend --image nginx --download
    karmor recommend --image nginx --overwrite-labels team=dev,env=prod
    karmor recommend --image nginx --append-labels owner=security,env=poc
    karmor recommend --image nginx --download --overwrite-labels team=dev,env=prod
    karmor recommend --image nginx --download --append-labels owner=security,env=poc

    // This might not be required.
    karmor recommend --image nginx --limit 2
    
Notes:
    1. The --image flag is mandatory.
    2. The --download flag will download all the policy files related to the image.
    3. The --overwrite-labels and --append-labels flags are mutually exclusive. Use only 1 at once`)
}

// downloadRecursive downloads files from a GitHub repository recursively
func downloadRecursive(repoName string, path string) error {
    apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repoName, path)
    resp, err := http.Get(apiURL)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    var items []GitHubFile
    if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
        return err
    }

    for _, item := range items {
        switch item.Type {
        case "file":
            localPath := filepath.Join("outputPolicy", item.Path)
            if err := downloadFile(item.DownloadURL, localPath); err != nil {
                return err
            }
        case "dir":
            if err := downloadRecursive(repoName, item.Path); err != nil {
                return err
            }
        }
    }
    return nil
}

// downloadFile downloads a file from a URL and saves it to a local path
func downloadFile(url string, localPath string) error {
    fmt.Println("⬇️ Downloading:", localPath)
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    err = os.MkdirAll(filepath.Dir(localPath), 0755)
    if err != nil {
        return err
    }

    out, err := os.Create(localPath)
    if err != nil {
        return err
    }
    defer out.Close()

    _, err = io.Copy(out, resp.Body)
    return err
}

// getDetailsFromArtifactHub fetches package details from Artifact Hub and stores them in a struct
func getDetailsFromArtifactHub(image string) (*SearchResponse, error) {
	query := url.QueryEscape(image)
	apiURL := fmt.Sprintf("https://artifacthub.io/api/v1/packages/search?kind=19&ts_query_web=%s&limit=1", query)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("❌ Failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	var data SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("❌ Failed to decode response: %w", err)
	}

	if len(data.Packages) == 0 {
		return nil, fmt.Errorf("❌ No packages found for the given image")
	}

	return &data, nil
}

// overwriteLabels overwrites all metadata.labels in a YAML file
func overwriteLabels(filePath string, newLabels map[string]string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var yamlMap map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlMap); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Traverse to spec.selector.matchLabels
	spec, ok := yamlMap["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no 'spec' field in YAML")
	}

	selector, ok := spec["selector"].(map[string]interface{})
	if !ok {
		selector = make(map[string]interface{})
		spec["selector"] = selector
	}

	matchLabels := make(map[string]interface{})
	for k, v := range newLabels {
		matchLabels[k] = v
	}
	selector["matchLabels"] = matchLabels

	// Re-marshal and write back
	updatedData, err := yaml.Marshal(yamlMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated YAML: %w", err)
	}

	return os.WriteFile(filePath, updatedData, 0644)
}

// parseLabels parses a comma-separated key=value string into a map
func parseLabels(labelStr string) (map[string]string, error) {
	labels := make(map[string]string)
	pairs := strings.Split(labelStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid label format: %s", pair)
		}
		labels[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return labels, nil
}

// applyOverwriteToAllYAMLs walks a folder and applies label overwrite to each YAML file
func applyOverwriteToAllYAMLs(rootDir string, newLabels map[string]string) error {
	return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a file
		if info.IsDir() {
			return nil
		}

		// Process only .yaml or .yml files
		if strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml") {
			fmt.Printf("📄 Overwriting labels in: %s\n", path)
			if err := overwriteLabels(path, newLabels); err != nil {
				fmt.Printf("❌ Error processing %s: %v\n", path, err)
			}
		}

		return nil
	})
}

// main function to handle command line arguments and call the appropriate functions
func main() {
    if len(os.Args) < 2 || os.Args[1] != "recommend" || (len(os.Args) > 2 && (os.Args[2] == "--help" || os.Args[2] == "-h")) {
        printHelp()
        return
    }

    // Subcommand: recommend
    recommendCmd := flag.NewFlagSet("recommend", flag.ExitOnError)
    image := recommendCmd.String("image", "", "Container image name")
    // label := recommendCmd.String("label", "", "Label selector")
    // limit := recommendCmd.Int("limit", 1, "Number of results to fetch (default: 1)")
    download := recommendCmd.Bool("download", false, "Download policy files")
    overwriteLabel := recommendCmd.String("overwrite-labels", "", "Overwrite spec.selector.matchLabels in the policy YAML with comma-separated labels (e.g. team=dev,env=prod)")
    appendLabel := recommendCmd.String("append-labels", "", "Append into spec.selector.matchLabels in the policy YAML with comma-separated labels (e.g. owner=security,env=poc)")

    // Parse only flags after "recommend"
    recommendCmd.Parse(os.Args[2:])

    // Logic
    // If image is not provided, error out
    if *image == "" {
        fmt.Println("❌ Image name is required. Use --image <image-name>")
        useHelp()
        os.Exit(1)
    }
    // If image is provided
    if *image != "" {
        // Get details related to provided image
        repoDetails, err := getDetailsFromArtifactHub(*image)
        if err != nil {
            fmt.Println(err)
            return
        }
        artifact := repoDetails.Packages[0]

        // Details from Artifact Hub using function getDetailsFromArtifactHub
        pkgName := artifact.Name
        pkgDisplayName := artifact.DisplayName
        pkgDescription := artifact.Description
        repoName := artifact.Repository.Name
        //repoOrgName := artifact.Repository.OrgName
        artifactHubURL := fmt.Sprintf("https://artifacthub.io/packages/kubearmor/%s/%s", repoName, pkgName)
        repoURL := artifact.Repository.URL
        // Strip "https://github.com/" from the start of the URL to get the repo name
        repoName = strings.TrimPrefix(repoURL, "https://github.com/")

        println("Package Display Name:", pkgDisplayName)
        println("Package Description:", pkgDescription)

        // If download is true
        if *download {

            fmt.Println("📦 Downloading policies for image:", *image)

            // Download the policy files if download is set to true
            err = downloadRecursive(repoName, *image)
            if err != nil {
                fmt.Println("❌ Error during download:", err)
            }

        } else {
            fmt.Printf("\nDisplaying recommendation for image: %s ➡️\n\n", *image)
            println("Artifact Hub URL:", artifactHubURL)
            fmt.Printf("\nFollow the above link to explore the Artifact Hub page or pass --download to get all policies related to %s\n", *image)
        }
        
        if *overwriteLabel != ""  {
            labelsMap, err := parseLabels(*overwriteLabel)
            if err != nil {
                fmt.Println("Invalid labels:", err)
                return
            }
    
            err = applyOverwriteToAllYAMLs("outputPolicy/nginx", labelsMap)
            if err != nil {
                fmt.Println("Failed to overwrite labels:", err)
            }
        }
        if *appendLabel != ""  {
            println("Append labels are not implemented yet.")
        }
    }
}

package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "text/tabwriter"
    "flag"
    "net/url"
    "sync"
    "github.com/schollz/progressbar/v3"
    "gopkg.in/yaml.v3"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constants for repeated strings
const (
    githubPrefix      = "https://github.com/"
    outputPolicyDir   = "outputPolicy"
    artifactHubBaseURL = "https://artifacthub.io/packages/kubearmor"
)

// Structs for Artifact Hub API response
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

// Helper function to display usage information
func printHelp() {
    fmt.Println(`karmor tool

Usage:
  karmor recommend --image <image-name> [--download] [ --overwrite-labels <key1=value1,key2=value2> | --append-labels <key1=value1,key2=value2> ] | --cluster

Flags:
  --image              Image name (e.g., nginx) (mandatory)
  --download           If passed, download the policy files (default: false)
  --overwrite-labels   Overwrite spec.selector.matchLabels in the policy YAML with comma-separated labels (e.g. team=dev,env=prod)
  --append-labels      Append into spec.selector.matchLabels in the policy YAML with comma-separated labels (e.g. owner=security,env=poc)
  --cluster            Fetch details from the cluster (default: false)
  --help, -h           To display all help options

Examples:
  karmor recommend --image nginx
  karmor recommend --image nginx --download
  karmor recommend --image nginx --overwrite-labels team=dev,env=prod
  karmor recommend --image nginx --append-labels owner=security,env=poc
  karmor recommend --image nginx --download --overwrite-labels team=dev,env=prod
  karmor recommend --image nginx --download --append-labels owner=security,env=poc
  karmor recommend --cluster

Notes:
  1. The --image flag is mandatory until --cluster is passed.
  2. The --download flag will download all the policy files related to the image.
  3. The --overwrite-labels and --append-labels flags are mutually exclusive. Use only 1 at once.`)
}

// Centralized error handling - unused right now
func handleError(err error, message string) {
    if err != nil {
        log.Fatalf("❌ %s: %v", message, err)
    }
}

// Fetch all policy image names from Artifact Hub
func fetchPolicyImageNames() (map[string]bool, error) {
    apiURL := "https://artifacthub.io/api/v1/packages/search?kind=19" // Adjust limit as needed

    // Make the HTTP GET request
    resp, err := http.Get(apiURL)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch policy data from Artifact Hub: %w", err)
    }
    defer resp.Body.Close()

    // Check for non-200 status codes
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch policy data: received status code %d", resp.StatusCode)
    }

    // Read the entire response body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %w", err)
    }

    // Parse the JSON response
    var data SearchResponse
    if err := json.Unmarshal(body, &data); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    // Create a map of image names for quick lookup
    policyMap := make(map[string]bool)
    for _, pkg := range data.Packages {
        policyMap[pkg.Name] = true
    }

    return policyMap, nil
}

// Fetch all container images along with resource type, pod name, and container name from the logged in cluster
func fetchClusterImagesWithDetails() ([][4]string, error) {
    // Create Kubernetes configuration
    config, err := getKubeConfig()
    if err != nil {
        return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
    }

    // Create a Kubernetes client
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
    }

    // Context for API calls
    ctx := context.TODO()

    // Slice to store image details (resource type, image name, pod name, container name)
    var imageDetails [][4]string

    // Fetch Deployments
    deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to list deployments: %w", err)
    }
    for _, deployment := range deployments.Items {
        for _, container := range deployment.Spec.Template.Spec.Containers {
            imageDetails = append(imageDetails, [4]string{"Deployment", container.Image, deployment.Name, container.Name})
        }
        for _, initContainer := range deployment.Spec.Template.Spec.InitContainers {
            imageDetails = append(imageDetails, [4]string{"Deployment", initContainer.Image, deployment.Name, initContainer.Name})
        }
    }

    // Fetch DaemonSets
    daemonSets, err := clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to list daemonsets: %w", err)
    }
    for _, daemonSet := range daemonSets.Items {
        for _, container := range daemonSet.Spec.Template.Spec.Containers {
            imageDetails = append(imageDetails, [4]string{"DaemonSet", container.Image, daemonSet.Name, container.Name})
        }
        for _, initContainer := range daemonSet.Spec.Template.Spec.InitContainers {
            imageDetails = append(imageDetails, [4]string{"DaemonSet", initContainer.Image, daemonSet.Name, initContainer.Name})
        }
    }

    // Fetch StatefulSets
    statefulSets, err := clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to list statefulsets: %w", err)
    }
    for _, statefulSet := range statefulSets.Items {
        for _, container := range statefulSet.Spec.Template.Spec.Containers {
            imageDetails = append(imageDetails, [4]string{"StatefulSet", container.Image, statefulSet.Name, container.Name})
        }
        for _, initContainer := range statefulSet.Spec.Template.Spec.InitContainers {
            imageDetails = append(imageDetails, [4]string{"StatefulSet", initContainer.Image, statefulSet.Name, initContainer.Name})
        }
    }

    // Fetch Pods (for standalone pods or Jobs)
    pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to list pods: %w", err)
    }
    for _, pod := range pods.Items {
        for _, container := range pod.Spec.Containers {
            imageDetails = append(imageDetails, [4]string{"Pod", container.Image, pod.Name, container.Name})
        }
        for _, initContainer := range pod.Spec.InitContainers {
            imageDetails = append(imageDetails, [4]string{"Pod", initContainer.Image, pod.Name, initContainer.Name})
        }
    }

    return imageDetails, nil
}

// Get Kubernetes configuration (handles both in-cluster and out-of-cluster configurations)
func getKubeConfig() (*rest.Config, error) {
    // Try in-cluster configuration
    config, err := rest.InClusterConfig()
    if err == nil {
        return config, nil
    }

    // Fallback to out-of-cluster configuration
    kubeconfig := os.Getenv("KUBECONFIG")
    if kubeconfig == "" {
        kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
    }

    config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
    if err != nil {
        return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
    }

    return config, nil
}

// Process image names to extract only the image name and remove registry name and version from the container images from cluster
func processImageName(image string) string {
    // Remove the registry name (everything before the last '/')
    lastSlashIndex := strings.LastIndex(image, "/")
    imageWithoutRegistry := image
    if lastSlashIndex != -1 {
        imageWithoutRegistry = image[lastSlashIndex+1:] // Take everything after the last '/'
    }

    // Remove the version or digest (everything after the ':' or '@')
    imageWithoutVersionOrDigest := strings.SplitN(imageWithoutRegistry, ":", 2)[0]
    imageWithoutVersionOrDigest = strings.SplitN(imageWithoutVersionOrDigest, "@", 2)[0]

    return imageWithoutVersionOrDigest
}

// Fetch particular package details from Artifact Hub for the KubeArmor policies based on the image provided on CLI
func getDetailsFromArtifactHub(image string) (*Package, error) {
    query := url.QueryEscape(image)
    apiURL := fmt.Sprintf("https://artifacthub.io/api/v1/packages/search?kind=19&ts_query_web=%s&limit=1", query)

    resp, err := http.Get(apiURL)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch details from Artifact Hub: %w", err)
    }
    defer resp.Body.Close()

    var data SearchResponse
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    if len(data.Packages) == 0 {
        return nil, fmt.Errorf("no polices found for the given image by KubeArmor")
    }

    return &data.Packages[0], nil
}

// Download the policy files recursively from the GitHub repository
func downloadRecursive(repoName, path string, progressBar *progressbar.ProgressBar) error {
    apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repoName, path)
    resp, err := http.Get(apiURL)
    if err != nil {
        return fmt.Errorf("failed to fetch repository contents: %w", err)
    }
    defer resp.Body.Close()

    // Check for non-200 status codes
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("failed to fetch repository contents: received status code %d for URL %s", resp.StatusCode, apiURL)
    }

    var items []GitHubFile
    if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
        return fmt.Errorf("failed to decode repository contents: %w", err)
    }

    for _, item := range items {
        progressBar.Add(1) // Increment the progress bar for each item processed

        switch item.Type {
        case "file":
            localPath := filepath.Join(outputPolicyDir, item.Path)
            if err := downloadFile(item.DownloadURL, localPath); err != nil {
                return fmt.Errorf("failed to download file %s: %w", item.Name, err)
            }
        case "dir":
            if err := downloadRecursive(repoName, item.Path, progressBar); err != nil {
                return err
            }
        }
    }
    return nil
}

// Download a single file
func downloadFile(fileURL, localPath string) error {
    resp, err := http.Get(fileURL)
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

// Parse labels to apply to policy yaml from a comma-separated string passed to the CLI
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

// Apply label overwrites to all YAML files in a directory (parallel processing)
func applyOverwriteToAllYAMLs(rootDir string, newLabels map[string]string) error {
    // Channel to send file paths to workers
    fileChan := make(chan string)
    // Channel to collect errors from workers
    errChan := make(chan error)
    // WaitGroup to wait for all workers to finish
    var wg sync.WaitGroup

    // Number of workers (adjust based on system resources)
    numWorkers := 4

    // Start workers
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for filePath := range fileChan {
                // Process the YAML file
                if err := overwriteLabels(filePath, newLabels); err != nil {
                    errChan <- fmt.Errorf("error processing %s: %w", filePath, err)
                } else {
                    fmt.Printf("✅ Updated file: %s\n", filePath) // Print updated file
                }
            }
        }()
    }

    // Walk the directory and send YAML file paths to the channel
    go func() {
        defer close(fileChan)
        err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
            if err != nil {
                return err
            }

            // Skip directories
            if info.IsDir() {
                return nil
            }

            // Process only .yaml or .yml files
            if strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml") {
                fileChan <- path
            }
            return nil
        })
        if err != nil {
            errChan <- err
        }
    }()

    // Close the error channel once all workers are done
    go func() {
        wg.Wait()
        close(errChan)
    }()

    // Collect errors
    var combinedErr error
    for err := range errChan {
        if combinedErr == nil {
            combinedErr = err
        } else {
            combinedErr = fmt.Errorf("%v; %w", combinedErr, err)
        }
    }

    return combinedErr
}

// Overwrite labels in a YAML file
func overwriteLabels(filePath string, newLabels map[string]string) error {
    data, err := os.ReadFile(filePath)
    if err != nil {
        return fmt.Errorf("failed to read file: %w", err)
    }

    var yamlMap map[string]interface{}
    if err := yaml.Unmarshal(data, &yamlMap); err != nil {
        return fmt.Errorf("failed to unmarshal YAML: %w", err)
    }

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

    updatedData, err := yaml.Marshal(yamlMap)
    if err != nil {
        return fmt.Errorf("failed to marshal updated YAML: %w", err)
    }

    return os.WriteFile(filePath, updatedData, 0644)
}

func deleteArtifactHubFile(rootDir string) error {
    // Walk the directory to find and delete artifacthub-pkg.yaml
    return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        // Check if the file is artifacthub-pkg.yaml
        if !info.IsDir() && strings.HasSuffix(info.Name(), "artifacthub-pkg.yaml") {
            return os.Remove(path)
        }
        return nil
    })
}

func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
        os.Exit(1)
    }
}

func parseAndValidateFlags() (*string, *bool, *string, *string, *bool, error) {
    recommendCmd := flag.NewFlagSet("recommend", flag.ContinueOnError)
    recommendCmd.SetOutput(io.Discard) // Suppress default usage output
    image := recommendCmd.String("image", "", "Container image name (optional for --cluster)")
    download := recommendCmd.Bool("download", false, "Download policy files")
    overwriteLabel := recommendCmd.String("overwrite-labels", "", "Overwrite spec.selector.matchLabels in the policy YAML")
    appendLabel := recommendCmd.String("append-labels", "", "Append into spec.selector.matchLabels in the policy YAML")
    cluster := recommendCmd.Bool("cluster", false, "Fetch details from the cluster")

    // Parse the flags
    err := recommendCmd.Parse(os.Args[2:])
    if err != nil {
        // Handle unknown or invalid flags
        if err == flag.ErrHelp {
            return nil, nil, nil, nil, nil, nil // Help flag was triggered, no error
        }
        return nil, nil, nil, nil, nil, fmt.Errorf("unrecognized option: %v\nUse --help for more information", err)
    }

    // Validate input flags
    if *cluster && (*overwriteLabel != "" || *appendLabel != "" || *image != "") {
        return nil, nil, nil, nil, nil, fmt.Errorf("invalid flag combination: --cluster cannot be combined with --image, --overwrite-labels, or --append-labels")
    }
    if !*cluster && *image == "" {
        return nil, nil, nil, nil, nil, fmt.Errorf("--image is mandatory unless --cluster is specified\nUse --help for more information")
    }
    if *overwriteLabel != "" && *appendLabel != "" {
        return nil, nil, nil, nil, nil, fmt.Errorf("--overwrite-labels and --append-labels are mutually exclusive")
    }

    return image, download, overwriteLabel, appendLabel, cluster, nil
}

func handleCluster(repoName string, download bool) error {
    // Fetch all policies from Artifact Hub
    policyMap, err := fetchPolicyImageNames()
    if err != nil {
        log.Fatalf("❌ Error fetching policies from Artifact Hub: %v", err)
    }
    // Fetch all cluster images with details
    imageDetails, err := fetchClusterImagesWithDetails()
    if err != nil {
        return fmt.Errorf("❌ Error fetching cluster images: %w", err)
    }

    // Use a map to track unique image names
    uniqueImages := make(map[string]bool)
    policyFoundBoolean := false // Flag to track if any policy was found

    // Print the table header
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.Debug)
    fmt.Fprintln(w, "RESOURCE TYPE\tPOD NAME\tCONTAINER NAME\tPROCESSED IMAGE NAME\tKUBEARMOR POLICY")

    // First pass: Process and print cluster details
    for _, detail := range imageDetails {
        imageName := detail[1] // The full image name
        if !uniqueImages[imageName] {
            // Add the image name to the map to mark it as seen
            uniqueImages[imageName] = true

            // Process the image name to extract only the base name
            processedImageName := processImageName(imageName)

            // Check if a policy exists for the processed image name
            hasPolicy := ""
            if policyMap[processedImageName] {
                hasPolicy = "true"
                policyFoundBoolean = true // Set the flag to true if any policy is found
            }

            // Print the details for the first occurrence of the image
            fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", detail[0], detail[2], detail[3], processedImageName, hasPolicy)
        }
    }
    // Flush the writer to ensure all output is printed
    w.Flush()

    // Print the message only if download is false
    if !download {
        fmt.Println("\nThe images in your cluster that have policies in Artifact Hub are listed above with the value True.\nPass --download flag to download the policy files for the image.\n")
    }

    // Second pass: Handle downloads
    if download {
        for _, detail := range imageDetails {
            imageName := detail[1] // The full image name
            processedImageName := processImageName(imageName)
            // Check if a policy exists for the processed image name
            if policyMap[processedImageName] {
                fmt.Printf("\n📦 Downloading policies for image: %s\n", processedImageName)
                progressBar := progressbar.NewOptions(-1, // -1 means unknown total count
                progressbar.OptionSetDescription("Downloading..."),
                progressbar.OptionShowCount(),
                progressbar.OptionShowBytes(false),
                progressbar.OptionSetWidth(40),
                progressbar.OptionSetTheme(progressbar.Theme{
                    Saucer:        "=",
                    SaucerPadding: " ",
                    BarStart:      "[",
                    BarEnd:        "]",
                    }),
                )
                if err := downloadRecursive(repoName, processedImageName, progressBar); err != nil {
                    return fmt.Errorf("error during download for image %s: %w", processedImageName, err)
                }
            }
        }
    }

    // If no policy was found, print a message
    if download && !policyFoundBoolean {
        fmt.Println("\n\nSorry, as of now we don't have any kubearmor policies for the images used in your cluster.")
    }

    return nil
}

func run() error {

    if len(os.Args) < 2 || os.Args[1] != "recommend" || (len(os.Args) > 2 && (os.Args[2] == "--help" || os.Args[2] == "-h")) {
        printHelp()
        return nil // No error, just showing help
    }

    // Parse and validate flags
    image, download, overwriteLabel, appendLabel, cluster, err := parseAndValidateFlags()
    if err != nil {
        return err
    }

    // Validate that only --cluster or --cluster + --download is passed
    if *cluster && (*overwriteLabel != "" || *appendLabel != "" || *image != "") {
        return fmt.Errorf("invalid flag combination: --cluster cannot be combined with --image, --overwrite-labels, or --append-labels")
    }

    // Fetch Artifact Hub details
    artifact, err := getDetailsFromArtifactHub(*image)
    if err != nil {
        return fmt.Errorf("failed to fetch details from Artifact Hub: %w", err)
    }

    if !strings.HasPrefix(artifact.Repository.URL, githubPrefix) {
        return fmt.Errorf("invalid repository URL format: %s", artifact.Repository.URL)
    }
    repoName := strings.TrimPrefix(artifact.Repository.URL, githubPrefix)

    // Handle different combinations of flags using a switch statement
    switch {    
    case *cluster && *download:
        // Case: --cluster + --download
        return handleCluster(repoName, true)

    case *cluster:
        // Case: --cluster only
        return handleCluster(repoName, false)

    case *image != "" && *download && *overwriteLabel != "":
        // Case: --image + --download + --overwrite-labels
        labelsMap, err := parseLabels(*overwriteLabel)
        if err != nil {
            return fmt.Errorf("invalid labels: %w", err)
        }

        fmt.Println("📦 Downloading policies for image:", *image)

        // Initialize the progress bar
        progressBar := progressbar.NewOptions(-1, // -1 means unknown total count
            progressbar.OptionSetDescription("Downloading..."),
            progressbar.OptionShowCount(),
            progressbar.OptionShowBytes(false),
            progressbar.OptionSetWidth(40),
            progressbar.OptionSetTheme(progressbar.Theme{
                Saucer:        "=",
                SaucerPadding: " ",
                BarStart:      "[",
                BarEnd:        "]",
            }),
        )

        if err := downloadRecursive(repoName, *image, progressBar); err != nil {
            return fmt.Errorf("error during download: %w", err)
        }

        if err := deleteArtifactHubFile(outputPolicyDir); err != nil {
            return fmt.Errorf("failed to delete artifacthub-pkg.yaml: %w", err)
        }

        if err := applyOverwriteToAllYAMLs(outputPolicyDir, labelsMap); err != nil {
            return fmt.Errorf("failed to overwrite labels: %w", err)
        }

    case *image != "" && *overwriteLabel != "":
        // Case: --image + --overwrite-labels
        // TODO: endpointSelector has some issue i guess check
        labelsMap, err := parseLabels(*overwriteLabel)
        if err != nil {
            return fmt.Errorf("invalid labels: %w", err)
        }

        if err := applyOverwriteToAllYAMLs(outputPolicyDir, labelsMap); err != nil {
            return fmt.Errorf("failed to overwrite labels: %w", err)
        }

    case *image != "" && *appendLabel != "":
        // Case: --image + --append-labels
        fmt.Println("Append labels are not implemented yet.")

    case *image != "" && *download:
        // Case: --image + --download
        fmt.Println("📦 Downloading policies for image:", *image)

        // Initialize the progress bar
        progressBar := progressbar.NewOptions(-1, // -1 means unknown total count
            progressbar.OptionSetDescription("Downloading..."),
            progressbar.OptionShowCount(),
            progressbar.OptionShowBytes(false),
            progressbar.OptionSetWidth(40),
            progressbar.OptionSetTheme(progressbar.Theme{
                Saucer:        "=",
                SaucerPadding: " ",
                BarStart:      "[",
                BarEnd:        "]",
            }),
        )

        if err := downloadRecursive(repoName, *image, progressBar); err != nil {
            return fmt.Errorf("error during download: %w", err)
        }

        if err := deleteArtifactHubFile(outputPolicyDir); err != nil {
            return fmt.Errorf("failed to delete artifacthub-pkg.yaml: %w", err)
        }

    default:
        // Case: --image only
        fmt.Println("Package Display Name:", artifact.DisplayName)
        fmt.Println("Package Description:", artifact.Description)
        fmt.Printf("\nArtifact Hub URL: %s/%s/%s\n", artifactHubBaseURL, repoName, artifact.Name)
        fmt.Printf("\nPass --download flag to download the policy files\n")
    }

    return nil // No errors
}

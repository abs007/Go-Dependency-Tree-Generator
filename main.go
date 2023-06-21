package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Artifact struct {
	Name         string `json:"Path"`
	Version      string `json:"Version"`
	Dependencies []*Artifact
}

type ArtifactWithDir struct {
	Artifact
	Dir string `json:"Dir"`
}

func main() {
	// Get the GitHub repo URL from the command line.
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run main.go <repo URL> <branch or tag> ")
		os.Exit(1)
	}
	repoLink := os.Args[1]
	branchOrTag := os.Args[2]

	// Clone repository
	dir, err := filepath.Abs(filepath.Base(repoLink))
	if err != nil {
		fmt.Println("Error: Failed to get absolute path for directory")
		os.Exit(1)
	}
	cmd := exec.Command("git", "clone", "-b", branchOrTag, repoLink, dir)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error: Failed to clone repository %s\n", repoLink)
		os.Exit(1)
	}

	// Navigate into directory
	if err := os.Chdir(dir); err != nil {
		fmt.Println("Error: Failed to navigate to directory")
		os.Exit(1)
	}

	// Downloading dependencies
	cmd = exec.Command("bash", "-c", "go mod download")
	if err := cmd.Run(); err != nil {
		fmt.Println("Error: Failed to download dependencies")
		os.Exit(1)
	}
	fmt.Println("Dependencies downloaded successfully")

	/////////////////////////////////////  Fetching dependencies and dependency's directory value  //////////////////////////////////////////////
	var tempartifacts []*Artifact
	var dirs []*string
	output, err := os.Getwd()
	if err != nil {
		fmt.Println("Error in fetching current directory:", err)
		os.Exit(1)
	}
	tempartifacts, dirs, err = fetchDependencies(output)
	if err != nil {
		fmt.Println("Error in fetching dependencies:", err)
		os.Exit(1)
	}

	println()
	for _, artifact := range tempartifacts {
		println("Name: ", artifact.Name, "Version: ", artifact.Version)
	}
	println()
	// Need to take care of null dependencies
	var artifactswithoutdir []*Artifact
	for i, artifact := range tempartifacts {
		err := populateDependencyTree(artifact, *dirs[i])
		if err != nil {
			panic(err)
		}
		artifactswithoutdir = append(artifactswithoutdir, artifact)
	}

	finalJSON, err := json.MarshalIndent(artifactswithoutdir, "", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Println(string(finalJSON))
}

// For each artifact passed and the corresponding directory value/location, add further sub dependencies
func populateDependencyTree(artifact *Artifact, Dir string) error {
	// Fetch immediate dependencies and their directory values for the given directory
	immediateDeps, dependencyDirectoryValues, err := fetchDependencies(Dir)
	if err != nil {
		return err
	}

	if immediateDeps == nil {
		artifact.Dependencies = []*Artifact{}
		return nil
	}

	// Iterate through immediate dependencies
	for i, dep := range immediateDeps {
		// Recursively populate the Dependencies field for each immediate dependency
		err := populateDependencyTree(dep, *dependencyDirectoryValues[i])
		if err != nil {
			return err
		}
	}

	// Assign the populated immediate dependencies to the artifact's Dependencies field
	artifact.Dependencies = immediateDeps

	return nil
}

// For fetching immediate dependencies and each dependency's directory value
// Returns nil if no go.mod file is present
func fetchDependencies(Dir string) ([]*Artifact, []*string, error) {

	os.Chdir(Dir)
	cmd := exec.Command("bash", "-c", "ls | grep go.mod")
	output, err := cmd.CombinedOutput()
	if string(output) == "" {
		return nil, []*string{}, nil
	}
	// Travelling to dependency directory and fetching further dependencies
	// If go.mod file is not present, then creating a new one and fetching dependencies
	cmd = exec.Command("bash", "-c", `go list -m -json all | jq -s "map(select(.Indirect != true and .Main != true))"`)
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error in running go list:", string(output))
		return nil, []*string{}, err
	}
	fmt.Println(os.Getwd())
	cmd = exec.Command("go", "mod", "tidy")
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error in running go mod tidy:", output)
		return nil, []*string{}, err
	}

	var artifactsWithDir []*ArtifactWithDir
	err = json.Unmarshal(output, &artifactsWithDir)
	if err != nil {
		fmt.Println(string(output))

		fmt.Println("Error with unmarshalling data:", err)
		return nil, []*string{}, err
	}

	var dependencyDirectoryValues []*string

	// Storing the immediate dependency's info in the Artifact struct and dependency's directory value in dependencyDirectoryValues
	// Dependencies field is still null here
	var artifactswithoutdir []*Artifact
	for _, artwithdir := range artifactsWithDir {
		artifactswithoutdir = append(artifactswithoutdir, &artwithdir.Artifact)
		dependencyDirectoryValues = append(dependencyDirectoryValues, &artwithdir.Dir)
	}

	return artifactswithoutdir, dependencyDirectoryValues, nil
}

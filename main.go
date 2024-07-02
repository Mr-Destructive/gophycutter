package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	configFileName = "cookiecutter.json"
	postGenDirName = "post_gen"
)

// generateContext reads and decodes the JSON configuration file into a context map.
func generateContext(configFile string) (map[string]interface{}, error) {
	context := make(map[string]interface{})

	file, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading JSON file: %v", err)
	}
	defer file.Close()

	var obj interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&obj); err != nil {
		return nil, fmt.Errorf("error decoding JSON file: %v", err)
	}

	context["cookiecutter"] = obj
	return context, nil
}

// generateFiles processes the input directory and generates files in the output directory.
func generateFiles(context map[string]interface{}, inputDir, outputDir string) error {
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing file or directory: %v", err)
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		relativePath, err := filepath.Rel(inputDir, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %v", err)
		}
		outputPath := filepath.Join(outputDir, renderTemplate(relativePath, context))

		if info.IsDir() {
			if err := os.MkdirAll(outputPath, os.ModePerm); err != nil {
				return fmt.Errorf("error creating directory: %v", err)
			}
		} else if info.Name() != configFileName {
			if err := processFile(path, outputPath, context); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	fmt.Println("Done!")
	return nil
}

// renderTemplate renders a template with the provided context.
func renderTemplate(input string, context map[string]interface{}) string {
	tmpl, err := template.New("").Parse(input)
	if err != nil {
		fmt.Printf("error parsing template: %v\n", err)
		return input
	}

	var renderedContent strings.Builder
	if err := tmpl.Execute(&renderedContent, context); err != nil {
		fmt.Printf("error executing template: %v\n", err)
		return input
	}
	return renderedContent.String()
}

// processFile reads, processes, and writes a file template.
func processFile(inputPath, outputPath string, context map[string]interface{}) error {
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("error reading input file: %v", err)
	}

	tmpl, err := template.New("").Parse(string(content))
	if err != nil {
		return fmt.Errorf("error parsing template: %v", err)
	}

	var renderedContent strings.Builder
	if err := tmpl.Execute(&renderedContent, context); err != nil {
		return fmt.Errorf("error rendering template: %v", err)
	}

	outputPath = renderTemplate(outputPath, context)

	if err := os.WriteFile(outputPath, []byte(renderedContent.String()), 0644); err != nil {
		return fmt.Errorf("error writing output file: %v", err)
	}

	return nil
}

// convertToType converts a string value to the specified type.
func convertToType(expectedType interface{}, value string) interface{} {
	switch expectedType.(type) {
	case string:
		return value
	case int:
		var v int
		fmt.Sscan(value, &v)
		return v
	case bool:
		var v bool
		fmt.Sscan(value, &v)
		return v
	case float64:
		var v float64
		fmt.Sscan(value, &v)
		return v
	default:
		return value
	}
}

// getUserInput reads user input from the console.
func getUserInput() string {
	var val string
	fmt.Scanln(&val)
	return val
}

// runPostGenScripts executes post-generation scripts in the output directory.
func runPostGenScripts(outputDir string) error {
	postGenDir := filepath.Join(outputDir, postGenDirName)
	_, err := os.Stat(postGenDir)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking post_gen directory: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(postGenDir, "*.go"))
	if err != nil {
		return fmt.Errorf("error listing files in post_gen directory: %v", err)
	}
	for _, file := range files {
		cmd := exec.Command("go", "run", file)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error running post-gen script %s: %v", file, err)
		}
	}
	return nil
}

func main() {
	var inputDir string
	fmt.Println("Enter the path to the directory:")
	fmt.Scanln(&inputDir)
	repoName := inputDir

	if strings.HasPrefix(inputDir, "https://") {
		repoUrl := strings.TrimRight(inputDir, "/")
		repoName = filepath.Base(repoUrl)
		err := exec.Command("git", "clone", inputDir).Run()
		if err != nil {
			fmt.Printf("error cloning repo: %v\n", err)
			return
		}
	}
	configFile := filepath.Join(repoName, configFileName)
	context, err := generateContext(configFile)
	if err != nil {
		fmt.Printf("error generating context: %v\n", err)
		return
	}

	data := context["cookiecutter"].(map[string]interface{})
	for key, value := range data {
		fmt.Printf("%v (%v): ", key, value)
		val := getUserInput()
		data[key] = convertToType(value, val)
	}
	context["cookiecutter"] = data

	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("error getting current directory: %v\n", err)
		return
	}
	outputDir := filepath.Join(currentDir, filepath.Base(repoName))
	err = generateFiles(context, inputDir, outputDir)
	if err != nil {
		fmt.Printf("error generating files: %v\n", err)
		return
	}
	err = runPostGenScripts(outputDir)
	if err != nil {
		fmt.Printf("error running post-gen scripts: %v\n", err)
		return
	}
}

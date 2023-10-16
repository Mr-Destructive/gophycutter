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

func generateContext(configFile string) (map[string]interface{}, error) {
	context := make(map[string]interface{})

	file, err := os.OpenFile(configFile, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("Error reading JSON file: %v", err)
	}
	defer file.Close()

	var obj interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&obj); err != nil {
		return nil, fmt.Errorf("Error decoding JSON file: %v", err)
	}

	context["cookiecutter"] = obj

	return context, nil
}

func generateFiles(context map[string]interface{}, inputDir, outputDir string) error {
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.MkdirAll(outputDir, os.ModePerm)
	}

	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("Error accessing file or directory: %v", err)
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		relativePath, _ := filepath.Rel(inputDir, path)
		outputPath := filepath.Join(outputDir, renderTemplate(relativePath, context))

		if info.IsDir() {
			if _, err := os.Stat(outputPath); os.IsNotExist(err) {
				os.MkdirAll(outputPath, os.ModePerm)
			}
		} else if info.Name() != "cookiecutter.json" {
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

func renderTemplate(input string, context map[string]interface{}) string {
	tmpl := template.New("").Funcs(template.FuncMap{})
	tmpl, _ = tmpl.Parse(input)

	var renderedContent strings.Builder
	tmpl.Execute(&renderedContent, context)
	return renderedContent.String()
}

func processFile(inputPath, outputPath string, context map[string]interface{}) error {
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("Error reading input file: %v", err)
	}

	tmpl := template.New("").Funcs(template.FuncMap{})

	_, err = tmpl.Parse(string(content))
	if err != nil {
		return fmt.Errorf("Error parsing template: %v", err)
	}

	var renderedContent strings.Builder

	err = tmpl.Execute(&renderedContent, context)
	if err != nil {
		return fmt.Errorf("Error rendering template: %v", err)
	}

	outputPath = renderTemplate(outputPath, context)

	err = os.WriteFile(outputPath, []byte(renderedContent.String()), 0644)
	if err != nil {
		return fmt.Errorf("Error writing output file: %v", err)
	}

	return nil
}

func convertToType(expectedType interface{}, value string) interface{} {
	switch expectedType.(type) {
	case string:
		if value == "" {
			return expectedType.(string)
		}
		return value
	case int:
		if value == "" {
			return expectedType.(int)
		}
		return value
	case bool:
		if value == "" {
			return expectedType.(bool)
		}
		return value
	case float64:
		if value == "" {
			return expectedType.(float64)
		}
		return value
	default:
		return value
	}
}

func getUserInput() string {
	var val string
	fmt.Scanln(&val)
	return val
}

func main() {
	var inputDir string
	fmt.Println("Enter the path to the directory:")
	fmt.Scanln(&inputDir)
	repoName := inputDir

	if strings.HasPrefix(inputDir, "https://") {
		// download the git repo
		repoUrl := strings.TrimRight(inputDir, "/")
		repoName = strings.Split(repoUrl, "/")[len(strings.Split(repoUrl, "/"))-1]
		err := exec.Command("git", "clone", inputDir).Run()
		if err != nil {
			fmt.Printf("Error cloning repo: %v\n", err)
			return
		}
	}
	configFile := filepath.Join(repoName, "cookiecutter.json")
	context, err := generateContext(configFile)
	if err != nil {
		fmt.Printf("Error generating context: %v\n", err)
		return
	}
	data := context["cookiecutter"].(map[string]interface{})
	for key, value := range data {
		fmt.Printf("%v (%v) : ", key, value)
		val := getUserInput()
		v := convertToType(value, val)
		data[key] = v
	}
	context["cookiecutter"] = data

	// get current working directory
	currentDir, err := os.Getwd()
	folderName := func() string {
		path := strings.Split(repoName, "/")
		return path[len(path)-1]
	}()
	outputDir := filepath.Join(currentDir, folderName)
	err = generateFiles(context, inputDir, outputDir)
	if err != nil {
		fmt.Printf("Error generating files: %v\n", err)
	}
}

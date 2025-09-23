package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

// SecretManager handles all secret operations
type SecretManager struct {
	secrets map[string]string
	missing []string
	masked  map[string]string
}

func main() {
	// Parse command line flags
	templateFile := flag.String("template", "Caddyfile", "Path to Caddyfile template")
	outputFile := flag.String("output", "Caddyfile.generated", "Path to output file")
	envFile := flag.String("env", ".env.local", "Path to environment file")
	useEnv := flag.Bool("use-env", true, "Load from environment variables")
	strict := flag.Bool("strict", false, "Fail if any placeholders are missing")
	preview := flag.Bool("preview", false, "Show preview with masked secrets")
	generateExample := flag.Bool("generate-example", false, "Generate .env.example file")
	updateWorkflow := flag.Bool("update-workflow", false, "Update GitHub Actions workflow")
	workflowFile := flag.String("workflow", ".github/workflows/deploy.yml", "Path to GitHub workflow")
	flag.Parse()

	// Handle different modes
	if *generateExample {
		if err := GenerateEnvExample(*templateFile, ".env.example"); err != nil {
			log.Fatalf("Failed to generate .env.example: %v", err)
		}
		fmt.Println("✅ Successfully generated .env.example")
		return
	}

	if *updateWorkflow {
		if err := UpdateGitHubWorkflow(*templateFile, *workflowFile); err != nil {
			log.Fatalf("Failed to update workflow: %v", err)
		}
		fmt.Printf("✅ Successfully updated %s\n", *workflowFile)
		return
	}

	// Normal generation mode
	sm := &SecretManager{
		secrets: make(map[string]string),
		missing: []string{},
		masked:  make(map[string]string),
	}

	// Load secrets from environment and/or file
	if *useEnv {
		sm.LoadFromEnvironment(*templateFile)
	}

	if *envFile != "" && fileExists(*envFile) {
		sm.LoadFromFile(*envFile)
	}

	if len(sm.secrets) == 0 {
		log.Fatal("❌ No secrets loaded. Please ensure .env.local exists or environment variables are set.")
	}

	// Process the template
	if err := sm.ProcessTemplate(*templateFile, *outputFile); err != nil {
		log.Fatalf("Failed to process template: %v", err)
	}

	// Show results
	if *preview {
		sm.PrintPreview(*outputFile)
	}

	// Check for missing secrets
	if len(sm.missing) > 0 && *strict {
		fmt.Printf("❌ Missing %d secrets:\n", len(sm.missing))
		for _, key := range sm.missing {
			fmt.Printf("  - %s\n", key)
		}
		os.Exit(1)
	}

	fmt.Printf("✅ Generated %s successfully\n", *outputFile)
	if len(sm.missing) > 0 {
		fmt.Printf("⚠️  %d placeholders were not replaced (use -strict to fail)\n", len(sm.missing))
	}
}

// LoadFromEnvironment loads secrets from environment variables based on template placeholders
func (sm *SecretManager) LoadFromEnvironment(templatePath string) {
	placeholders, err := DiscoverPlaceholders(templatePath)
	if err != nil {
		log.Printf("Warning: Could not discover placeholders: %v", err)
		return
	}

	loaded := 0
	for _, placeholder := range placeholders {
		if value := os.Getenv(placeholder); value != "" {
			sm.secrets[placeholder] = value
			sm.masked[placeholder] = MaskSecret(value)
			loaded++
		}
	}

	if loaded > 0 {
		fmt.Printf("📦 Loaded %d secrets from environment\n", loaded)
	}
}

// LoadFromFile loads secrets from a .env file
func (sm *SecretManager) LoadFromFile(filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Warning: Could not read env file: %v", err)
		return
	}

	loaded := 0
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			sm.secrets[key] = value
			sm.masked[key] = MaskSecret(value)
			loaded++
		}
	}

	if loaded > 0 {
		fmt.Printf("📦 Loaded %d secrets from %s\n", loaded, filePath)
	}
}

// ProcessTemplate replaces placeholders in the template
func (sm *SecretManager) ProcessTemplate(templatePath, outputPath string) error {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	result := string(content)
	re := regexp.MustCompile(`\{\{([A-Z_0-9]+)\}\}`)

	result = re.ReplaceAllStringFunc(result, func(match string) string {
		key := match[2 : len(match)-2] // Remove {{ and }}
		if value, exists := sm.secrets[key]; exists {
			return value
		}
		sm.missing = append(sm.missing, key)
		return match // Keep placeholder if not found
	})

	return os.WriteFile(outputPath, []byte(result), 0644)
}

// PrintPreview shows a masked preview of the generated file
func (sm *SecretManager) PrintPreview(outputPath string) error {
	fmt.Println("\n📋 Preview (secrets masked):")
	fmt.Println("=" + strings.Repeat("=", 50))

	for key, masked := range sm.masked {
		fmt.Printf("  %s = %s\n", key, masked)
		if len(sm.masked) > 10 {
			fmt.Println("  ... (showing first 10)")
			break
		}
	}

	fmt.Println("=" + strings.Repeat("=", 50))
	return nil
}

// DiscoverPlaceholders finds all placeholders in a template file
func DiscoverPlaceholders(templatePath string) ([]string, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`\{\{([A-Z_0-9]+)\}\}`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	seen := make(map[string]bool)
	var placeholders []string
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			placeholders = append(placeholders, match[1])
		}
	}

	sort.Strings(placeholders)
	return placeholders, nil
}

// MaskSecret masks a secret value for display
func MaskSecret(value string) string {
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + strings.Repeat("*", len(value)-4)
}

// GenerateEnvExample creates an .env.example file from template placeholders
func GenerateEnvExample(templatePath, outputPath string) error {
	placeholders, err := DiscoverPlaceholders(templatePath)
	if err != nil {
		return err
	}

	var lines []string
	lines = append(lines, "# Auto-generated from Caddyfile template")
	lines = append(lines, "# Copy this file to .env.local and replace with actual values")
	lines = append(lines, "# DO NOT COMMIT .env.local TO VERSION CONTROL")
	lines = append(lines, "")

	// Group by prefix
	grouped := make(map[string][]string)
	for _, key := range placeholders {
		prefix := strings.Split(key, "_")[0]
		grouped[prefix] = append(grouped[prefix], key)
	}

	// Sort prefixes
	var prefixes []string
	for prefix := range grouped {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	// Generate example values
	for _, prefix := range prefixes {
		lines = append(lines, fmt.Sprintf("# === %s Keys ===", prefix))
		sort.Strings(grouped[prefix])
		for _, key := range grouped[prefix] {
			// Generate dummy value based on key pattern
			dummyValue := "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
			if strings.Contains(key, "KEY") && strings.Contains(key, "_") {
				parts := strings.Split(key, "_")
				if len(parts) > 0 && (parts[len(parts)-1] == "1" || parts[len(parts)-1] == "2") {
					dummyValue = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
				}
			}
			lines = append(lines, fmt.Sprintf("%s=%s", key, dummyValue))
		}
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return os.WriteFile(outputPath, []byte(content), 0644)
}

// UpdateGitHubWorkflow updates the GitHub Actions workflow with current secrets
func UpdateGitHubWorkflow(templatePath, workflowPath string) error {
	// Discover placeholders
	placeholders, err := DiscoverPlaceholders(templatePath)
	if err != nil {
		return err
	}

	// Read workflow
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	inEnvSection := false
	envIndent := ""
	sectionStart := -1
	sectionEnd := -1

	// Find the env section under "Generate Caddyfile from secrets"
	foundGenerate := false
	for i, line := range lines {
		if strings.Contains(line, "name: Generate Caddyfile from secrets") {
			foundGenerate = true
		}
		if foundGenerate && strings.TrimSpace(line) == "env:" {
			inEnvSection = true
			envIndent = strings.Replace(line, "env:", "", 1)
			sectionStart = i
			continue
		}
		if inEnvSection && (strings.Contains(line, "run:") ||
			(len(strings.TrimSpace(line)) > 0 &&
				!strings.HasPrefix(strings.TrimSpace(line), "#") &&
				!strings.Contains(line, "${{"))) {
			sectionEnd = i
			break
		}
	}

	if sectionStart == -1 || sectionEnd == -1 {
		return fmt.Errorf("could not find env section")
	}

	// Build new env section
	var newSection []string
	newSection = append(newSection, lines[sectionStart])
	newSection = append(newSection, envIndent+"  # Auto-generated secret mappings")
	newSection = append(newSection, envIndent+"  # Run 'make secrets-update-ci' to regenerate")

	for _, key := range placeholders {
		newSection = append(newSection, fmt.Sprintf("%s  %s: ${{ secrets.%s }}", envIndent, key, key))
	}

	// Reconstruct file
	result = append(result, lines[:sectionStart]...)
	result = append(result, newSection...)
	result = append(result, lines[sectionEnd:]...)

	return os.WriteFile(workflowPath, []byte(strings.Join(result, "\n")), 0644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

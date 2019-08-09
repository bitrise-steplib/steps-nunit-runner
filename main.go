package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-steputils/input"
	steptools "github.com/bitrise-tools/go-steputils/tools"
	"github.com/bitrise-tools/go-xamarin/builder"
	"github.com/bitrise-tools/go-xamarin/constants"
	"github.com/bitrise-tools/go-xamarin/tools"
	"github.com/bitrise-tools/go-xamarin/tools/buildtools"
	shellquote "github.com/kballard/go-shellquote"
)

// ConfigsModel ...
type ConfigsModel struct {
	XamarinSolution      string
	XamarinConfiguration string
	XamarinPlatform      string

	CustomOptions string

	BuildTool      string
	BuildBeforeRun string
	DeployDir      string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		XamarinSolution:      os.Getenv("xamarin_solution"),
		XamarinConfiguration: os.Getenv("xamarin_configuration"),
		XamarinPlatform:      os.Getenv("xamarin_platform"),

		CustomOptions: os.Getenv("nunit_options"),

		BuildTool:      os.Getenv("build_tool"),
		BuildBeforeRun: os.Getenv("build_before_test"),
		DeployDir:      os.Getenv("BITRISE_DEPLOY_DIR"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Configs:")

	log.Printf("- XamarinSolution: %s", configs.XamarinSolution)
	log.Printf("- XamarinConfiguration: %s", configs.XamarinConfiguration)
	log.Printf("- XamarinPlatform: %s", configs.XamarinPlatform)

	log.Infof("Debug:")

	log.Printf("- BuildBeforeTest: %s", configs.BuildBeforeRun)
	log.Printf("- CustomOptions: %s", configs.CustomOptions)
	log.Printf("- BuildTool: %s", configs.BuildTool)
	log.Printf("- DeployDir: %s", configs.DeployDir)
}

func (configs ConfigsModel) validate() error {
	if err := input.ValidateIfPathExists(configs.XamarinSolution); err != nil {
		return fmt.Errorf("XamarinSolution - %s", err)
	}
	if err := input.ValidateIfNotEmpty(configs.XamarinConfiguration); err != nil {
		return fmt.Errorf("XamarinConfiguration - %s", err)
	}
	if err := input.ValidateIfNotEmpty(configs.XamarinPlatform); err != nil {
		return fmt.Errorf("XamarinPlatform - %s", err)
	}

	if err := input.ValidateWithOptions(configs.BuildBeforeRun, "true", "false"); err != nil {
		return fmt.Errorf("BuildBeforeRun - %s", err)
	}
	if err := input.ValidateWithOptions(configs.BuildTool, "msbuild", "xbuild"); err != nil {
		return fmt.Errorf("BuildTool - %s", err)
	}

	return nil
}

func testResultLogContent(pth string) (string, error) {
	if exist, err := pathutil.IsPathExists(pth); err != nil {
		return "", fmt.Errorf("Failed to check if path (%s) exist, error: %s", pth, err)
	} else if !exist {
		return "", fmt.Errorf("test result not exist at: %s", pth)
	}

	content, err := fileutil.ReadStringFromFile(pth)
	if err != nil {
		return "", fmt.Errorf("Failed to read file (%s), error: %s", pth, err)
	}

	return content, nil
}

func main() {
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		log.Errorf("Issue with input: %s", err)

		if err := steptools.ExportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
		}

		os.Exit(1)
	}

	// Custom Options
	resultLogPth := filepath.Join(configs.DeployDir, "TestResult.xml")
	customOptions := []string{"--result", resultLogPth}
	if configs.CustomOptions != "" {
		options, err := shellquote.Split(configs.CustomOptions)
		if err != nil {
			log.Errorf("Failed to split params (%s), error: %s", configs.CustomOptions, err)

			if err := steptools.ExportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
				log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
			}

			os.Exit(1)
		}

		customOptions = append(customOptions, options...)
	}
	// ---

	//
	// build
	fmt.Println()
	log.Infof("Running all nunit test projects in solution: %s", configs.XamarinSolution)

	buildTool := buildtools.Msbuild
	if configs.BuildTool == "xbuild" {
		buildTool = buildtools.Xbuild
	}

	builder, err := builder.New(configs.XamarinSolution, []constants.SDK{}, buildTool)
	if err != nil {
		log.Errorf("Failed to create xamarin builder, error: %s", err)

		if err := steptools.ExportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
		}

		os.Exit(1)
	}

	prepareCallback := func(solutionName string, projectName string, sdk constants.SDK, projectType constants.TestFramework, command *tools.Editable) {
		if projectType == constants.TestFrameworkNunitTest {
			(*command).SetCustomOptions(customOptions...)
		}
	}

	callback := func(solutionName string, projectName string, sdk constants.SDK, projectType constants.TestFramework, commandStr string, alreadyPerformed bool) {
		fmt.Println()
		if projectName == "" {
			log.Infof("Building solution: %s", solutionName)
		} else {
			if projectType == constants.TestFrameworkNunitTest {
				log.Infof("Running test project: %s", projectName)
			} else {
				log.Infof("Building project: %s", projectName)
			}
		}

		log.Donef("$ %s", commandStr)

		if alreadyPerformed {
			log.Warnf("build command already performed, skipping...")
		}

		fmt.Println()
	}

	var warnings []string
	err = nil

	if configs.BuildBeforeRun == "true" {
		warnings, err = builder.BuildAndRunAllNunitTestProjects(configs.XamarinConfiguration, configs.XamarinPlatform, callback, prepareCallback)
	} else {
		warnings, err = builder.RunAllNunitTestProjects(configs.XamarinConfiguration, configs.XamarinPlatform, callback, prepareCallback)
	}

	for _, warning := range warnings {
		log.Warnf(warning)
	}

	if err != nil {
		log.Errorf("Test run failed, error: %s", err)

		if err := steptools.ExportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
		}

		os.Exit(1)
	}

	resultLog, logErr := testResultLogContent(resultLogPth)
	if logErr != nil {
		log.Warnf("Failed to read test result, error: %s", logErr)
	}

	if expErr := steptools.ExportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "succeeded"); expErr != nil {
		log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", expErr)
	}

	if resultLog != "" {
		if expErr := steptools.ExportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", resultLog); expErr != nil {
			log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", expErr)
		}
	}
}

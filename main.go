package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cmdex "github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-xamarin/builder"
	"github.com/bitrise-tools/go-xamarin/constants"
	"github.com/bitrise-tools/go-xamarin/tools"
	shellquote "github.com/kballard/go-shellquote"
)

// ConfigsModel ...
type ConfigsModel struct {
	XamarinSolution      string
	XamarinConfiguration string
	XamarinPlatform      string

	CustomOptions string

	BuildBeforeRun string
	DeployDir      string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		XamarinSolution:      os.Getenv("xamarin_solution"),
		XamarinConfiguration: os.Getenv("xamarin_configuration"),
		XamarinPlatform:      os.Getenv("xamarin_platform"),

		CustomOptions: os.Getenv("nunit_options"),

		BuildBeforeRun: os.Getenv("build_before_test"),
		DeployDir:      os.Getenv("BITRISE_DEPLOY_DIR"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Build Configs:")

	log.Printf("- XamarinSolution: %s", configs.XamarinSolution)
	log.Printf("- XamarinConfiguration: %s", configs.XamarinConfiguration)
	log.Printf("- XamarinPlatform: %s", configs.XamarinPlatform)

	log.Infof("Nunit Configs:")

	log.Printf("- CustomOptions: %s", configs.CustomOptions)

	log.Infof("Other Configs:")

	log.Printf("- BuildBeforeTest: %s", configs.BuildBeforeRun)
	log.Printf("- DeployDir: %s", configs.DeployDir)
}

func (configs ConfigsModel) validate() error {
	if configs.XamarinSolution == "" {
		return errors.New("no XamarinSolution parameter specified")
	}
	if exist, err := pathutil.IsPathExists(configs.XamarinSolution); err != nil {
		return fmt.Errorf("Failed to check if XamarinSolution exist at: %s, error: %s", configs.XamarinSolution, err)
	} else if !exist {
		return fmt.Errorf("XamarinSolution not exist at: %s", configs.XamarinSolution)
	}

	if configs.BuildBeforeRun != "true" && configs.BuildBeforeRun != "false" {
		return fmt.Errorf("invalid BuildBeforeRun parameter, provided: %s, available: [yes, no]", configs.BuildBeforeRun)
	}

	if configs.XamarinConfiguration == "" {
		return errors.New("no XamarinConfiguration parameter specified")
	}
	if configs.XamarinPlatform == "" {
		return errors.New("no XamarinPlatform parameter specified")
	}

	return nil
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	cmd := cmdex.New("envman", "add", "--key", keyStr)
	cmd.SetStdin(strings.NewReader(valueStr))
	return cmd.Run()
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

		if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
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

			if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
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

	builder, err := builder.New(configs.XamarinSolution, []constants.SDK{}, false)
	if err != nil {
		log.Errorf("Failed to create xamarin builder, error: %s", err)

		if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
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

	warnings := []string{}
	err = nil

	if configs.BuildBeforeRun == "true" {
		warnings, err = builder.BuildAndRunAllNunitTestProjects(configs.XamarinConfiguration, configs.XamarinPlatform, callback, prepareCallback)
	} else {
		warnings, err = builder.RunAllNunitTestProjects(configs.XamarinConfiguration, configs.XamarinPlatform, callback, prepareCallback)
	}

	resultLog, logErr := testResultLogContent(resultLogPth)
	if logErr != nil {
		log.Warnf("Failed to read test result, error: %s", logErr)
	}

	for _, warning := range warnings {
		log.Warnf(warning)
	}

	if err != nil {
		log.Errorf("Test run failed, error: %s", err)

		if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
		}

		if resultLog != "" {
			if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", resultLog); err != nil {
				log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", err)
			}
		}

		os.Exit(1)
	}

	if expErr := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "succeeded"); expErr != nil {
		log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", expErr)
	}

	if resultLog != "" {
		if expErr := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", resultLog); expErr != nil {
			log.Warnf("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", expErr)
		}
	}
}

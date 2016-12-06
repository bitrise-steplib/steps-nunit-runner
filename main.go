package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/cmdex"
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

	DeployDir string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		XamarinSolution:      os.Getenv("xamarin_solution"),
		XamarinConfiguration: os.Getenv("xamarin_configuration"),
		XamarinPlatform:      os.Getenv("xamarin_platform"),

		CustomOptions: os.Getenv("nunit_options"),

		DeployDir: os.Getenv("BITRISE_DEPLOY_DIR"),
	}
}

func (configs ConfigsModel) print() {
	log.Info("Build Configs:")

	log.Detail("- XamarinSolution: %s", configs.XamarinSolution)
	log.Detail("- XamarinConfiguration: %s", configs.XamarinConfiguration)
	log.Detail("- XamarinPlatform: %s", configs.XamarinPlatform)

	log.Info("Nunit Configs:")

	log.Detail("- CustomOptions: %s", configs.CustomOptions)

	log.Info("Other Configs:")

	log.Detail("- DeployDir: %s", configs.DeployDir)
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

	if configs.XamarinConfiguration == "" {
		return errors.New("no XamarinConfiguration parameter specified")
	}
	if configs.XamarinPlatform == "" {
		return errors.New("no XamarinPlatform parameter specified")
	}

	return nil
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	cmd := cmdex.NewCommand("envman", "add", "--key", keyStr)
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
		log.Error("Issue with input: %s", err)

		if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
			log.Warn("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
		}

		os.Exit(1)
	}

	// Custom Options
	resultLogPth := filepath.Join(configs.DeployDir, "TestResult.xml")
	customOptions := []string{"--result", resultLogPth}
	if configs.CustomOptions != "" {
		options, err := shellquote.Split(configs.CustomOptions)
		if err != nil {
			log.Error("Failed to split params (%s), error: %s", configs.CustomOptions, err)

			if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
				log.Warn("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
			}

			os.Exit(1)
		}

		customOptions = append(customOptions, options...)
	}
	// ---

	//
	// build
	fmt.Println()
	log.Info("Runing all nunit test projects in solution: %s", configs.XamarinSolution)

	builder, err := builder.New(configs.XamarinSolution, []constants.ProjectType{}, false)
	if err != nil {
		log.Error("Failed to create xamarin builder, error: %s", err)

		if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
			log.Warn("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
		}

		os.Exit(1)
	}

	prepareCallback := func(solutionName string, projectName string, projectType constants.ProjectType, command *tools.Editable) {
		if projectType == constants.ProjectTypeNunitTest {
			(*command).SetCustomOptions(customOptions...)
		}
	}

	callback := func(solutionName string, projectName string, projectType constants.ProjectType, commandStr string, alreadyPerformed bool) {
		fmt.Println()
		if projectName == "" {
			log.Info("Building solution: %s", solutionName)
		} else {
			if projectType == constants.ProjectTypeNunitTest {
				log.Info("Building test project: %s", projectName)
			} else {
				log.Info("Building project: %s", projectName)
			}
		}

		log.Done("$ %s", commandStr)

		if alreadyPerformed {
			log.Warn("build command already performed, skipping...")
		}

		fmt.Println()
	}

	warnings, err := builder.BuildAllNunitTestProjects(configs.XamarinConfiguration, configs.XamarinPlatform, prepareCallback, callback)
	resultLog, logErr := testResultLogContent(resultLogPth)
	if logErr != nil {
		log.Warn("Failed to read test result, error: %s", logErr)
	}

	for _, warning := range warnings {
		log.Warn(warning)
	}

	if err != nil {
		log.Error("Test run failed, error: %s", err)

		if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "failed"); err != nil {
			log.Warn("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
		}

		if resultLog != "" {
			if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", resultLog); err != nil {
				log.Warn("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", err)
			}
		}

		os.Exit(1)
	}

	if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_RESULT", "succeeded"); err != nil {
		log.Warn("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_RESULT", err)
	}

	if resultLog != "" {
		if err := exportEnvironmentWithEnvman("BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", resultLog); err != nil {
			log.Warn("Failed to export environment: %s, error: %s", "BITRISE_XAMARIN_TEST_FULL_RESULTS_TEXT", err)
		}
	}
}

package client

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/alessio/shellescape"
	"github.com/blang/semver/v4"
	"github.com/fatih/color"
	"github.com/mitchellh/go-homedir"
	"github.com/rocket-pool/smartnode/v2/shared/config"
)

const (
	installerName              string = "install.sh"
	updateTrackerInstallerName string = "install-update-tracker.sh"
	installerURL               string = "https://github.com/rocket-pool/smartnode/releases/download/%s/" + installerName
	updateTrackerURL           string = "https://github.com/rocket-pool/smartnode/releases/download/%s/" + updateTrackerInstallerName

	debugColor         color.Attribute = color.FgYellow
	nethermindAdminUrl string          = "http://127.0.0.1:7434"
)

func (c *Client) downloadAndRun(
	name string,
	url string,
	verbose bool,
	version string,
	extraFlags []string,
) error {
	var script []byte

	// Download the installation script
	resp, err := http.Get(fmt.Sprintf(url, version))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected http status downloading %s script: %d", name, resp.StatusCode)
	}

	// Sanity check that the script octet length matches content-length
	script, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if fmt.Sprint(len(script)) != resp.Header.Get("content-length") {
		return fmt.Errorf("downloaded script length %d did not match content-length header %s", len(script), resp.Header.Get("content-length"))
	}

	return c.runScript(script, version, verbose, extraFlags)
}

func (c *Client) runScript(
	script []byte,
	version string,
	verbose bool,
	extraFlags []string,
) error {

	flags := []string{
		"-v", shellescape.Quote(version),
	}
	flags = append(flags, extraFlags...)

	// Get the escalation command
	escalationCmd, err := c.getEscalationCommand()
	if err != nil {
		return fmt.Errorf("error getting escalation command: %w", err)
	}

	// Initialize installation command
	cmd := c.newCommand(fmt.Sprintf("%s sh -s -- %s", escalationCmd, strings.Join(flags, " ")))

	// Pass the script to sh via its stdin fd
	cmd.SetStdin(bytes.NewReader(script))

	// Get command output pipes
	cmdOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmdErr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Print progress from stdout
	go (func() {
		scanner := bufio.NewScanner(cmdOut)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	})()

	// Read command & error output from stderr; render in verbose mode
	var errMessage string
	go (func() {
		c := color.New(debugColor)
		scanner := bufio.NewScanner(cmdErr)
		for scanner.Scan() {
			errMessage = scanner.Text()
			if verbose {
				_, _ = c.Println(scanner.Text())
			}
		}
	})()

	// Run command and return error output
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("could not install Smart Node service: %s", errMessage)
	}
	return nil
}

func readLocalScript(path string) ([]byte, error) {
	// Make sure it exists
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("local script [%s] does not exist", path)
	}
	if err != nil {
		return nil, fmt.Errorf("error checking script [%s]: %w", path, err)
	}

	// Read it
	script, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading local script [%s]: %w", path, err)
	}

	return script, nil
}

// Install the Rocket Pool service
// installScriptPath is optional. If unset, the install script is downloaded from github.
func (c *Client) InstallService(verbose bool, noDeps bool, version string, path string, installScriptPath string) error {

	// Get installation script flags
	flags := []string{}
	if path != "" {
		flags = append(flags, fmt.Sprintf("-p %s", shellescape.Quote(path)))
	}
	if noDeps {
		flags = append(flags, "-d")
	}

	if installScriptPath != "" {
		script, err := readLocalScript(installScriptPath)
		if err != nil {
			return err
		}
		// Set the "local mode" flag
		flags = append(flags, "-l")
		return c.runScript(script, version, verbose, flags)
	}

	return c.downloadAndRun(installerName, installerURL, verbose, version, flags)
}

// Install the update tracker
func (c *Client) InstallUpdateTracker(verbose bool, version string, installScriptPath string) error {
	if installScriptPath != "" {
		script, err := readLocalScript(installScriptPath)
		if err != nil {
			return err
		}
		return c.runScript(script, version, verbose, nil)
	}
	return c.downloadAndRun(updateTrackerInstallerName, updateTrackerURL, verbose, version, nil)
}

// Start the Rocket Pool service
func (c *Client) StartService(composeFiles []string) error {
	cmd, err := c.compose(composeFiles, "up -d --remove-orphans --quiet-pull")
	if err != nil {
		return err
	}
	return c.printOutput(cmd)
}

// Pause the Rocket Pool service
func (c *Client) PauseService(composeFiles []string) error {
	cmd, err := c.compose(composeFiles, "stop")
	if err != nil {
		return err
	}
	return c.printOutput(cmd)
}

// Stop the Rocket Pool service
func (c *Client) StopService(composeFiles []string) error {
	cmd, err := c.compose(composeFiles, "down -v")
	if err != nil {
		return err
	}
	return c.printOutput(cmd)
}

// Stop the Rocket Pool service and remove the config folder
func (c *Client) TerminateService(composeFiles []string, configPath string) error {
	// Get the command to run with root privileges
	rootCmd, err := c.getEscalationCommand()
	if err != nil {
		return fmt.Errorf("could not get privilege escalation command: %w", err)
	}

	// Terminate the Docker containers
	cmd, err := c.compose(composeFiles, "down -v")
	if err != nil {
		return fmt.Errorf("error creating Docker artifact removal command: %w", err)
	}
	err = c.printOutput(cmd)
	if err != nil {
		return fmt.Errorf("error removing Docker artifacts: %w", err)
	}

	// Delete the RP directory
	path, err := homedir.Expand(configPath)
	if err != nil {
		return fmt.Errorf("error loading Rocket Pool directory: %w", err)
	}
	fmt.Printf("Deleting Rocket Pool directory (%s)...\n", path)
	cmd = fmt.Sprintf("%s rm -rf %s", rootCmd, path)
	_, err = c.readOutput(cmd)
	if err != nil {
		return fmt.Errorf("error deleting Rocket Pool directory: %w", err)
	}

	fmt.Println("Termination complete.")

	return nil
}

// Print the Rocket Pool service status
func (c *Client) PrintServiceStatus(composeFiles []string) error {
	cmd, err := c.compose(composeFiles, "ps")
	if err != nil {
		return err
	}
	return c.printOutput(cmd)
}

// Print the Rocket Pool service logs
func (c *Client) PrintServiceLogs(composeFiles []string, tail string, serviceNames ...string) error {
	sanitizedStrings := make([]string, len(serviceNames))
	for i, serviceName := range serviceNames {
		sanitizedStrings[i] = shellescape.Quote(serviceName)
	}
	cmd, err := c.compose(composeFiles, fmt.Sprintf("logs -f --tail %s %s", shellescape.Quote(tail), strings.Join(sanitizedStrings, " ")))
	if err != nil {
		return err
	}
	return c.printOutput(cmd)
}

// Print the daemon logs
func (c *Client) PrintNodeLogs(composeFiles []string, tail string, logPaths ...string) error {
	cmd := fmt.Sprintf("tail -f %s %s", tail, strings.Join(logPaths, " "))
	return c.printOutput(cmd)
}

// Print the Rocket Pool service stats
func (c *Client) PrintServiceStats(composeFiles []string) error {
	// Get service container IDs
	cmd, err := c.compose(composeFiles, "ps -q")
	if err != nil {
		return err
	}
	containers, err := c.readOutput(cmd)
	if err != nil {
		return err
	}
	containerIds := strings.Split(strings.TrimSpace(string(containers)), "\n")

	// Print stats
	return c.printOutput(fmt.Sprintf("docker stats %s", strings.Join(containerIds, " ")))
}

// Print the Rocket Pool service compose config
func (c *Client) PrintServiceCompose(composeFiles []string) error {
	cmd, err := c.compose(composeFiles, "config")
	if err != nil {
		return err
	}
	return c.printOutput(cmd)
}

// Get the Rocket Pool service version
func (c *Client) GetServiceVersion() (string, error) {
	// Get service container version output
	response, err := c.Api.Service.Version()
	if err != nil {
		return "", fmt.Errorf("error requesting Rocket Pool service version: %w", err)
	}
	versionString := response.Data.Version

	// Make sure it's a semantic version
	version, err := semver.Make(versionString)
	if err != nil {
		return "", fmt.Errorf("error parsing Rocket Pool service version number from output '%s': %w", versionString, err)
	}

	// Return the parsed semantic version (extra safety)
	return version.String(), nil
}

// Deletes the node wallet and all validator keys, and restarts the Docker containers
func (c *Client) PurgeAllKeys(composeFiles []string) error {
	// Get the command to run with root privileges
	rootCmd, err := c.getEscalationCommand()
	if err != nil {
		return fmt.Errorf("could not get privilege escalation command: %w", err)
	}

	// Get the config
	cfg, _, err := c.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading user settings: %w", err)
	}

	// Check for Native mode
	if cfg.IsNativeMode {
		return fmt.Errorf("this function is not supported in Native Mode; you will have to shut down your client and daemon services and remove the keys manually")
	}

	// Shut down the containers
	fmt.Println("Stopping containers...")
	err = c.PauseService(composeFiles)
	if err != nil {
		return fmt.Errorf("error stopping Docker containers: %w", err)
	}

	// Delete the address
	nodeAddressPath, err := homedir.Expand(cfg.GetNodeAddressPath())
	if err != nil {
		return fmt.Errorf("error loading node address file path: %w", err)
	}
	fmt.Println("Deleting node address file...")
	cmd := fmt.Sprintf("%s rm -f %s", rootCmd, nodeAddressPath)
	_, err = c.readOutput(cmd)
	if err != nil {
		return fmt.Errorf("error deleting node address file: %w", err)
	}

	// Delete the wallet
	walletPath, err := homedir.Expand(cfg.GetWalletPath())
	if err != nil {
		return fmt.Errorf("error loading wallet path: %w", err)
	}
	fmt.Println("Deleting wallet...")
	cmd = fmt.Sprintf("%s rm -f %s", rootCmd, walletPath)
	_, err = c.readOutput(cmd)
	if err != nil {
		return fmt.Errorf("error deleting wallet: %w", err)
	}

	// Delete the next account file
	nextAccountPath, err := homedir.Expand(cfg.GetNextAccountFilePath())
	if err != nil {
		return fmt.Errorf("error loading next account file path: %w", err)
	}
	fmt.Println("Deleting next account file...")
	cmd = fmt.Sprintf("%s rm -f %s", rootCmd, nextAccountPath)
	_, err = c.readOutput(cmd)
	if err != nil {
		return fmt.Errorf("error deleting next account file: %w", err)
	}

	// Delete the password
	passwordPath, err := homedir.Expand(cfg.GetPasswordPath())
	if err != nil {
		return fmt.Errorf("error loading password path: %w", err)
	}
	fmt.Println("Deleting password...")
	cmd = fmt.Sprintf("%s rm -f %s", rootCmd, passwordPath)
	_, err = c.readOutput(cmd)
	if err != nil {
		return fmt.Errorf("error deleting password: %w", err)
	}

	// Delete the validators dir
	validatorsPath, err := homedir.Expand(cfg.GetValidatorsFolderPath())
	if err != nil {
		return fmt.Errorf("error loading validators folder path: %w", err)
	}
	fmt.Println("Deleting validator keys...")
	cmd = fmt.Sprintf("%s rm -rf %s/*", rootCmd, validatorsPath)
	_, err = c.readOutput(cmd)
	if err != nil {
		return fmt.Errorf("error deleting validator keys: %w", err)
	}
	cmd = fmt.Sprintf("%s rm -rf %s/.[a-zA-Z0-9]*", rootCmd, validatorsPath)
	_, err = c.readOutput(cmd)
	if err != nil {
		return fmt.Errorf("error deleting hidden files in validator folder: %w", err)
	}

	// Start the containers
	fmt.Println("Starting containers...")
	err = c.StartService(composeFiles)
	if err != nil {
		return fmt.Errorf("error starting Docker containers: %w", err)
	}

	fmt.Println("Purge complete.")

	return nil
}

// Runs the prune provisioner
func (c *Client) RunPruneProvisioner(container string, volume string) error {
	// Run the prune provisioner
	cmd := fmt.Sprintf("docker run --rm --name %s -v %s:/ethclient %s", container, volume, config.PruneProvisionerTag)
	output, err := c.readOutput(cmd)
	if err != nil {
		return err
	}

	outputString := strings.TrimSpace(string(output))
	if outputString != "" {
		return fmt.Errorf("Unexpected output running the prune provisioner: %s", outputString)
	}

	return nil
}

// Runs the prune provisioner
func (c *Client) RunNethermindPruneStarter(executionContainerName string, pruneStarterContainerName string) error {
	cmd := fmt.Sprintf(`docker run --rm --name %s --network container:%s rocketpool/nm-prune-starter %s`, pruneStarterContainerName, executionContainerName, nethermindAdminUrl)
	err := c.printOutput(cmd)
	if err != nil {
		return err
	}
	return nil
}

// Runs the EC migrator
func (c *Client) RunEcMigrator(container string, volume string, targetDir string, mode string) error {
	cmd := fmt.Sprintf("docker run --rm --name %s -v %s:/ethclient -v %s:/mnt/external -e EC_MIGRATE_MODE='%s' %s", container, volume, targetDir, mode, config.EcMigratorTag)
	err := c.printOutput(cmd)
	if err != nil {
		return err
	}

	return nil
}

// Gets the size of the target directory via the EC migrator for importing, which should have the same permissions as exporting
func (c *Client) GetDirSizeViaEcMigrator(container string, targetDir string) (uint64, error) {
	cmd := fmt.Sprintf("docker run --rm --name %s -v %s:/mnt/external -e OPERATION='size' %s", container, targetDir, config.EcMigratorTag)
	output, err := c.readOutput(cmd)
	if err != nil {
		return 0, fmt.Errorf("Error getting source directory size: %w", err)
	}

	trimmedOutput := strings.TrimRight(string(output), "\n")
	dirSize, err := strconv.ParseUint(trimmedOutput, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("Error parsing directory size output [%s]: %w", trimmedOutput, err)
	}

	return dirSize, nil
}

// Create the user config directory
func (c *Client) CreateUserDir() error {
	// Create the user directory
	err := os.MkdirAll(c.Context.ConfigPath, 0700)
	if err != nil {
		return fmt.Errorf("error creating config path [%s]: %w", c.Context.ConfigPath, err)
	}

	return nil
}
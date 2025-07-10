package ssh

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
	"mysql-backup/internal/config"
)

type Client struct {
	config *config.SSHConfig
	client *ssh.Client
}

func NewClient(sshConfig *config.SSHConfig) *Client {
	return &Client{
		config: sshConfig,
	}
}

func (c *Client) Connect() error {
	var auth []ssh.AuthMethod

	// SSH Key Authentication
	if c.config.PrivateKey != "" {
		var keyData []byte
		var err error

		if c.config.KeyPath != "" {
			// Read from file path
			keyData, err = ioutil.ReadFile(c.config.KeyPath)
			if err != nil {
				return fmt.Errorf("failed to read private key file: %w", err)
			}
		} else {
			// Use inline private key
			keyData = []byte(c.config.PrivateKey)
		}

		var signer ssh.Signer
		if c.config.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(c.config.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}

		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}

		auth = append(auth, ssh.PublicKeys(signer))
		fmt.Printf("SSH: Using key authentication for user %s\n", c.config.Username)
	}

	// SSH Password Authentication (fallback)
	if len(auth) == 0 && c.config.Password != "" {
		auth = append(auth, ssh.Password(c.config.Password))
		fmt.Printf("SSH: Using password authentication for user %s\n", c.config.Username)
	}

	if len(auth) == 0 {
		return fmt.Errorf("no SSH authentication method available (need private key or password)")
	}

	clientConfig := &ssh.ClientConfig{
		User:            c.config.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	fmt.Printf("SSH: Connecting to %s as user %s\n", address, c.config.Username)
	
	client, err := ssh.Dial("tcp", address, clientConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server %s: %w", address, err)
	}

	c.client = client
	fmt.Printf("SSH: Successfully connected to %s\n", address)
	return nil
}

func (c *Client) TestConnection() error {
	if err := c.Connect(); err != nil {
		return err
	}
	defer c.Close()

	// Test connection by running a simple command
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.Output("echo 'SSH connection test successful'")
	if err != nil {
		return fmt.Errorf("failed to execute test command: %w", err)
	}

	if len(output) == 0 {
		return fmt.Errorf("SSH test command returned empty output")
	}

	fmt.Printf("SSH: Test successful - %s\n", string(output))
	return nil
}

func (c *Client) ExecuteCommand(command string) ([]byte, error) {
	if c.client == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	return session.Output(command)
}

func (c *Client) CreateTunnel(localPort, remoteHost string, remotePort int) (net.Listener, error) {
	if c.client == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	localListener, err := net.Listen("tcp", fmt.Sprintf("localhost:%s", localPort))
	if err != nil {
		return nil, fmt.Errorf("failed to create local listener: %w", err)
	}

	fmt.Printf("SSH Tunnel: Local port %s -> Remote %s:%d\n", localPort, remoteHost, remotePort)

	// Test if we can connect to the remote MySQL server first
	fmt.Printf("SSH Tunnel: Testing remote MySQL connection...\n")
	testConn, err := c.client.Dial("tcp", fmt.Sprintf("%s:%d", remoteHost, remotePort))
	if err != nil {
		localListener.Close()
		return nil, fmt.Errorf("cannot connect to remote MySQL %s:%d through SSH: %w", remoteHost, remotePort, err)
	}
	testConn.Close()
	fmt.Printf("SSH Tunnel: Remote MySQL connection test successful\n")

	go func() {
		for {
			localConn, err := localListener.Accept()
			if err != nil {
				fmt.Printf("SSH Tunnel: Local listener closed: %v\n", err)
				return
			}

			go c.handleTunnelConnection(localConn, remoteHost, remotePort)
		}
	}()

	return localListener, nil
}

func (c *Client) handleTunnelConnection(localConn net.Conn, remoteHost string, remotePort int) {
	defer localConn.Close()

	fmt.Printf("SSH Tunnel: New connection from %s\n", localConn.RemoteAddr())

	// Connect to remote MySQL through SSH
	remoteConn, err := c.client.Dial("tcp", fmt.Sprintf("%s:%d", remoteHost, remotePort))
	if err != nil {
		fmt.Printf("SSH Tunnel: Failed to connect to remote %s:%d - %v\n", remoteHost, remotePort, err)
		return
	}
	defer remoteConn.Close()

	fmt.Printf("SSH Tunnel: Connected to remote %s:%d\n", remoteHost, remotePort)

	// Copy data bidirectionally
	done := make(chan bool, 2)

	// Copy from local to remote
	go func() {
		defer func() { done <- true }()
		bytes, err := io.Copy(remoteConn, localConn)
		if err != nil {
			fmt.Printf("SSH Tunnel: Error copying local->remote: %v\n", err)
		} else {
			fmt.Printf("SSH Tunnel: Copied %d bytes local->remote\n", bytes)
		}
	}()

	// Copy from remote to local
	go func() {
		defer func() { done <- true }()
		bytes, err := io.Copy(localConn, remoteConn)
		if err != nil {
			fmt.Printf("SSH Tunnel: Error copying remote->local: %v\n", err)
		} else {
			fmt.Printf("SSH Tunnel: Copied %d bytes remote->local\n", bytes)
		}
	}()

	// Wait for one direction to finish
	<-done
	fmt.Printf("SSH Tunnel: Connection closed\n")
}

func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

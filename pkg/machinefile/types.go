package internal

type Runner interface {
	RunCommand(command string, userName string, envVars map[string]string) error
	CopyFile(srcPattern, dest string, isAdd bool) error
}

type LocalRunner struct {
	BaseDir string
}

type SSHRunner struct {
	BaseDir     string
	SshHost     string
	SshUser     string
	SshKeyPath  string
	SshPassword string
	AskPassword bool
}

package service

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/subprocess"

	log "github.com/sirupsen/logrus"
)

type Contester struct {
	InvokerId     string
	Sandboxes     []SandboxPair
	Env           []*contester_proto.LocalEnvironment_Variable
	ServerAddress string

	Platform     string
	Disks        []string
	ProgramFiles []string

	// DebugAddress mirrors contesterConfig.DebugAddress; see its doc.
	DebugAddress string

	// NoUserProfile mirrors contesterConfig.NoUserProfile; see its doc.
	NoUserProfile bool

	// UseHandleList mirrors contesterConfig.UseHandleList; see its doc.
	UseHandleList bool

	// NoLogonInherit mirrors contesterConfig.NoLogonInherit; see its doc.
	NoLogonInherit bool

	// UseAsUser is set at startup when the invoker holds the privileges to
	// launch restricted runs via CreateProcessAsUser with the cached logon
	// token, avoiding a per-execute CreateProcessWithLogonW. See tryEnableAsUser.
	UseAsUser bool

	// UseWithToken is set at startup when UseAsUser is unavailable but the
	// invoker holds SeImpersonatePrivilege, so restricted runs can use
	// CreateProcessWithTokenW from the cached token (no per-execute logon).
	// See tryEnableWithToken.
	UseWithToken bool

	GData *platform.GlobalData
}

func getHostname() string {
	if result, err := os.Hostname(); err == nil {
		return result
	}
	return "undefined"
}

func getLocalEnvironment() []*contester_proto.LocalEnvironment_Variable {
	list := os.Environ()
	result := make([]*contester_proto.LocalEnvironment_Variable, 0, len(list))
	for _, v := range list {
		s := strings.SplitN(v, "=", 2)
		result = append(result, &contester_proto.LocalEnvironment_Variable{
			Name:  s[0],
			Value: s[1]})
	}
	return result
}

func newSandboxPair(base string) SandboxPair {
	return SandboxPair{
		Compile: &Sandbox{
			Path: filepath.Join(base, "C"),
		},
		Run: &Sandbox{
			Path: filepath.Join(base, "R"),
		},
	}
}

func configureSandboxes(config *contesterConfig) ([]SandboxPair, error) {
	basePath := config.Path
	passwords := getPasswords(config)
	result := make([]SandboxPair, len(passwords))
	for index, password := range passwords {
		localBase := filepath.Join(basePath, strconv.Itoa(index))
		result[index] = newSandboxPair(localBase)

		e := checkSandbox(result[index].Compile.Path)
		if e != nil {
			return nil, e
		}
		e = checkSandbox(result[index].Run.Path)
		if e != nil {
			return nil, e
		}

		if PLATFORM_ID == "linux" {
			e = setAcl(result[index].Compile.Path, "compiler")
			if e != nil {
				return nil, e
			}
			result[index].Compile.Login, e = subprocess.NewLoginInfo("compiler", "compiler")
			if e != nil {
				return nil, e
			}
		}

		restrictedUser := "tester" + strconv.Itoa(index)

		if err := ensureRestrictedUser(restrictedUser, password); err != nil {
			return nil, err
		}

		e = setAcl(result[index].Run.Path, restrictedUser)
		if e != nil {
			return nil, e
		}
		// HACK HACK: on linux, passwords are ignored.
		result[index].Run.Login, e = subprocess.NewLoginInfo(restrictedUser, password)
		if e != nil {
			if !isLogonFailure(e) {
				return nil, e
			}
			log.Infof("Logon failure for %q, trying to reset password", restrictedUser)
			if err := maybeResetPassword(restrictedUser, password); err != nil {
				return nil, err
			}
			result[index].Run.Login, e = subprocess.NewLoginInfo(restrictedUser, password)
			if e != nil {
				return nil, e
			}
		}
	}
	return result, nil
}

func checkSandbox(path string) error {
	err := os.MkdirAll(path, os.ModeDir|0755)
	if err != nil {
		return err
	}
	return nil
}

type contesterConfig struct {
	Server, Passwords, Path string
	SandboxCount            int
	// NoUserProfile, when set, runs restricted solutions through
	// CreateProcessWithLogonW with dwLogonFlags=0 instead of
	// LOGON_WITH_PROFILE, skipping the per-execute user-profile hive load.
	// Off by default: without a profile the solution's HKCU and
	// %USERPROFILE%/%TEMP% fall back to the default profile, which some
	// runtimes depend on.
	NoUserProfile bool `toml:"no_user_profile"`
	// UseHandleList, when set, makes privileged (CreateProcessAsUser) launches
	// inherit stdio through an explicit STARTUPINFOEX handle list instead of
	// the global inheritable-handle set, so they no longer serialize on
	// syscall.ForkLock. Off by default; effective only where CreateProcessAsUser
	// is in use (the privilege is held). See subprocess.Subprocess.UseHandleList.
	UseHandleList bool `toml:"use_handle_list"`
	// NoLogonInherit drops the inherit-marking and ForkLock from the
	// CreateProcessWithLogonW path (unprivileged launches); see
	// subprocess.Subprocess.NoLogonInherit. Off by default; verify stdio still
	// reaches WithLogonW children before enabling.
	NoLogonInherit bool `toml:"no_logon_inherit"`
	// DisableCreateProcessWithToken turns off the CreateProcessWithTokenW
	// fallback (used when the SeAssignPrimaryToken privilege for
	// CreateProcessAsUser is absent but SeImpersonate is held). Off by default,
	// so an admin box automatically uses the no-logon token path. See
	// subprocess.Subprocess.UseWithToken.
	DisableCreateProcessWithToken bool `toml:"disable_create_process_with_token"`
	// DebugAddress, when set (e.g. "127.0.0.1:6060"), starts a net/http/pprof
	// server there for profiling (/debug/pprof/goroutine, /mutex, /block).
	// Off by default; bind to localhost or a trusted network — it exposes
	// profiling endpoints. See runner main.
	DebugAddress string `toml:"debug_address"`
}

func NewContester(configFile string, gData *platform.GlobalData) (*Contester, error) {
	var config contesterConfig
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		return nil, err
	}

	log.Infof("Loaded contester config %+v", &config)

	// Choose the restricted-launch path once, by privilege: CreateProcessAsUser
	// if SeAssignPrimaryToken is held, else CreateProcessWithTokenW if
	// SeImpersonate is held (and not disabled), else CreateProcessWithLogonW.
	useAsUser := tryEnableAsUser()
	useWithToken := false
	if !useAsUser {
		useWithToken = tryEnableWithToken(config.DisableCreateProcessWithToken)
	}

	result := Contester{
		InvokerId:      getHostname(),
		Env:            getLocalEnvironment(),
		ServerAddress:  config.Server,
		Platform:       PLATFORM_ID,
		Disks:          PLATFORM_DISKS,
		ProgramFiles:   PLATFORM_PFILES,
		DebugAddress:   config.DebugAddress,
		NoUserProfile:  config.NoUserProfile,
		UseHandleList:  config.UseHandleList,
		NoLogonInherit: config.NoLogonInherit,
		UseAsUser:      useAsUser,
		UseWithToken:   useWithToken,
		GData:          gData,
	}

	var err error
	result.Sandboxes, err = configureSandboxes(&config)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *Contester) Identify(request *contester_proto.IdentifyRequest, response *contester_proto.IdentifyResponse) error {
	response.InvokerId = s.InvokerId
	response.Environment = &contester_proto.LocalEnvironment{
		Variable: s.Env}
	response.Sandboxes = make([]*contester_proto.SandboxLocations, 0, len(s.Sandboxes))
	for _, p := range s.Sandboxes {
		response.Sandboxes = append(response.Sandboxes, &contester_proto.SandboxLocations{
			Compile: p.Compile.Path,
			Run:     p.Run.Path})
	}
	response.Platform = s.Platform
	response.PathSeparator = string(os.PathSeparator)
	response.Disks = s.Disks
	response.ProgramFiles = s.ProgramFiles

	return nil
}

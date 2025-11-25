package service

import (
	"sync"

	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/subprocess"
)

func fillEnv(src *contester_proto.LocalEnvironment) ([]string, bool) {
	if src == nil {
		return nil, false
	}

	result := make([]string, 0, len(src.Variable))
	for _, v := range src.Variable {
		result = append(result, v.GetName()+"="+v.GetValue())
	}
	return result, true
}

func parseSuccessCode(succ uint32) *contester_proto.ExecutionResultFlags {
	if succ == 0 {
		return nil
	}
	return &contester_proto.ExecutionResultFlags{
		Killed:                 succ&subprocess.EF_KILLED != 0,
		TimeLimitHit:           succ&subprocess.EF_TIME_LIMIT_HIT != 0,
		KernelTimeLimitHit:     succ&subprocess.EF_KERNEL_TIME_LIMIT_HIT != 0,
		WallTimeLimitHit:       succ&subprocess.EF_WALL_TIME_LIMIT_HIT != 0,
		MemoryLimitHit:         succ&subprocess.EF_MEMORY_LIMIT_HIT != 0,
		Inactive:               succ&subprocess.EF_INACTIVE != 0,
		TimeLimitHitPost:       succ&subprocess.EF_TIME_LIMIT_HIT_POST != 0,
		KernelTimeLimitHitPost: succ&subprocess.EF_KERNEL_TIME_LIMIT_HIT_POST != 0,
		MemoryLimitHitPost:     succ&subprocess.EF_MEMORY_LIMIT_HIT_POST != 0,
		ProcessLimitHit:        succ&subprocess.EF_PROCESS_LIMIT_HIT != 0,
	}
}

func parseTime(r *subprocess.SubprocessResult) *contester_proto.ExecutionResultTime {
	if r.UserTime == 0 && r.KernelTime == 0 && r.WallTime == 0 {
		return nil
	}

	var result contester_proto.ExecutionResultTime

	if r.UserTime != 0 {
		result.UserTimeMicros = subprocess.GetMicros(r.UserTime)
	}
	if r.KernelTime != 0 {
		result.KernelTimeMicros = subprocess.GetMicros(r.KernelTime)
	}
	if r.WallTime != 0 {
		result.WallTimeMicros = subprocess.GetMicros(r.WallTime)
	}
	return &result
}

func fillRedirect(r *contester_proto.RedirectParameters) *subprocess.Redirect {
	if r == nil {
		return nil
	}

	var result subprocess.Redirect
	if r.GetFilename() != "" {
		result.Filename = r.GetFilename()
		result.Mode = subprocess.REDIRECT_FILE
	} else if r.GetMemory() {
		result.Mode = subprocess.REDIRECT_MEMORY
		if r.Buffer != nil {
			result.Data, _ = r.Buffer.Bytes()
		}
	}
	return &result
}

func findSandbox(s []SandboxPair, request *contester_proto.LocalExecutionParameters) (*Sandbox, error) {
	if request.GetSandboxId() != "" {
		return getSandboxById(s, request.GetSandboxId())
	}
	return getSandboxByPath(s, request.GetCurrentDirectory())
}

func fillResult(result *subprocess.SubprocessResult, response *contester_proto.LocalExecutionResult) {
	response.TotalProcesses = result.TotalProcesses
	response.ReturnCode = result.ExitCode
	response.Flags = parseSuccessCode(result.SuccessCode)
	response.Time = parseTime(result)
	response.Memory = result.PeakMemory
	response.StdOut, _ = contester_proto.NewBlob(result.Output)
	response.StdErr, _ = contester_proto.NewBlob(result.Error)
}

func (s *Contester) setupSubprocess(request *contester_proto.LocalExecutionParameters, sandbox *Sandbox, doRedirects bool) (sub *subprocess.Subprocess, err error) {
	sub = subprocess.SubprocessCreate()

	sub.Cmd = &subprocess.CommandLine{
		ApplicationName: request.ApplicationName,
		CommandLine:     request.CommandLine,
		Parameters:      request.CommandLineParameters,
	}

	sub.CurrentDirectory = request.CurrentDirectory

	sub.TimeLimit = subprocess.DuFromMicros(request.GetTimeLimitMicros())
	sub.KernelTimeLimit = subprocess.DuFromMicros(request.GetKernelTimeLimitMicros())
	sub.WallTimeLimit = subprocess.DuFromMicros(request.GetWallTimeLimitMicros())
	sub.MemoryLimit = request.GetMemoryLimit()
	sub.CheckIdleness = request.GetCheckIdleness()
	sub.RestrictUi = request.GetRestrictUi()
	sub.NoJob = request.GetNoJob()

	sub.Environment, sub.NoInheritEnvironment = fillEnv(request.Environment)

	if doRedirects {
		sub.StdIn = fillRedirect(request.StdIn)
		sub.StdOut = fillRedirect(request.StdOut)
	}

	if request.GetJoinStdoutStderr() {
		sub.JoinStdOutErr = true
	} else {
		sub.StdErr = fillRedirect(request.StdErr)
	}

	sub.Options = &subprocess.PlatformOptions{}

	if sandbox.Login != nil {
		sub.Login = sandbox.Login
	} else {
		if PLATFORM_ID == "linux" {
			sub.Login, err = subprocess.NewLoginInfo("compiler", "compiler")
			if err != nil {
				return
			}
		}
	}

	err = s.localPlatformSetup(sub, request)
	return
}

func chmodRequestIfNeeded(sandbox *Sandbox, request *contester_proto.LocalExecutionParameters) error {
	if request.GetApplicationName() != "" {
		return chmodIfNeeded(request.GetApplicationName(), sandbox)
	}
	return nil
}

func (s *Contester) LocalExecute(request *contester_proto.LocalExecutionParameters, response *contester_proto.LocalExecutionResult) error {
	sandbox, err := findSandbox(s.Sandboxes, request)
	if err != nil {
		return err
	}

	sandbox.Mutex.Lock()
	defer sandbox.Mutex.Unlock()

	err = chmodRequestIfNeeded(sandbox, request)
	if err != nil {
		return err
	}

	sub, err := s.setupSubprocess(request, sandbox, true)

	if err != nil {
		return err
	}

	result, err := sub.Execute()

	if err != nil {
		return err
	}

	fillResult(result, response)

	return nil
}

func (s *Contester) LocalExecuteConnected(request *contester_proto.LocalExecuteConnected, response *contester_proto.LocalExecuteConnectedResult) error {
	firstSandbox, err := findSandbox(s.Sandboxes, request.First)
	if err != nil {
		return err
	}

	secondSandbox, err := findSandbox(s.Sandboxes, request.Second)
	if err != nil {
		return err
	}

	firstSandbox.Mutex.Lock()
	defer firstSandbox.Mutex.Unlock()

	secondSandbox.Mutex.Lock()
	defer secondSandbox.Mutex.Unlock()

	err = chmodRequestIfNeeded(firstSandbox, request.First)
	if err != nil {
		return err
	}

	err = chmodRequestIfNeeded(secondSandbox, request.Second)
	if err != nil {
		return err
	}

	first, err := s.setupSubprocess(request.First, firstSandbox, false)
	if err != nil {
		return err
	}

	second, err := s.setupSubprocess(request.Second, secondSandbox, false)
	if err != nil {
		return err
	}

	err = subprocess.Interconnect(first, second, nil, nil, nil, nil)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var e1, e2 error

	runaway := func(sp *subprocess.Subprocess, ep *error, cp **contester_proto.LocalExecutionResult) {
		defer wg.Done()
		r, e := sp.Execute()
		if e != nil {
			*ep = e
			return
		}
		*cp = &contester_proto.LocalExecutionResult{}
		fillResult(r, *cp)
	}

	wg.Add(2)
	go runaway(first, &e1, &response.First)
	go runaway(second, &e2, &response.Second)

	wg.Wait()

	if e1 != nil {
		return e1
	}
	if e2 != nil {
		return e2
	}
	return nil
}

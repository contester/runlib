package service

import (
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/subprocess"
	"github.com/golang/protobuf/proto"
)

func fillEnv(src *contester_proto.LocalEnvironment) *[]string {
	if src == nil {
		return nil
	}

	result := make([]string, len(src.Variable))
	for i, v := range src.Variable {
		result[i] = v.GetName() + "=" + v.GetValue()
	}
	return &result
}

func parseSuccessCode(succ uint32) *contester_proto.ExecutionResultFlags {
	if succ == 0 {
		return nil
	}
	result := &contester_proto.ExecutionResultFlags{}
	if succ&subprocess.EF_KILLED != 0 {
		result.Killed = proto.Bool(true)
	}
	if succ&subprocess.EF_TIME_LIMIT_HIT != 0 {
		result.TimeLimitHit = proto.Bool(true)
	}
	if succ&subprocess.EF_MEMORY_LIMIT_HIT != 0 {
		result.MemoryLimitHit = proto.Bool(true)
	}
	if succ&subprocess.EF_INACTIVE != 0 {
		result.Inactive = proto.Bool(true)
	}
	if succ&subprocess.EF_TIME_LIMIT_HARD != 0 {
		result.TimeLimitHard = proto.Bool(true)
	}
	if succ&subprocess.EF_TIME_LIMIT_HIT_POST != 0 {
		result.TimeLimitHitPost = proto.Bool(true)
	}
	if succ&subprocess.EF_MEMORY_LIMIT_HIT_POST != 0 {
		result.MemoryLimitHitPost = proto.Bool(true)
	}
	if succ&subprocess.EF_PROCESS_LIMIT_HIT != 0 {
		result.ProcessLimitHit = proto.Bool(true)
	}

	return result
}

func parseTime(r *subprocess.SubprocessResult) *contester_proto.ExecutionResultTime {
	if r.UserTime == 0 && r.KernelTime == 0 && r.WallTime == 0 {
		return nil
	}

	result := &contester_proto.ExecutionResultTime{}

	if r.UserTime != 0 {
		result.UserTimeMicros = proto.Uint64(subprocess.GetMicros(r.UserTime))
	}
	if r.KernelTime != 0 {
		result.KernelTimeMicros = proto.Uint64(subprocess.GetMicros(r.KernelTime))
	}
	if r.WallTime != 0 {
		result.WallTimeMicros = proto.Uint64(subprocess.GetMicros(r.WallTime))
	}
	return result
}

func fillRedirect(r *contester_proto.RedirectParameters) *subprocess.Redirect {
	if r == nil {
		return nil
	}

	result := &subprocess.Redirect{}
	if r.Filename != nil {
		result.Filename = r.Filename
		result.Mode = subprocess.REDIRECT_FILE
	} else if r.Memory != nil && *r.Memory {
		result.Mode = subprocess.REDIRECT_MEMORY
		if r.Buffer != nil {
			result.Data, _ = r.Buffer.Bytes()
		}
	}
	return result
}

func findSandbox(s []SandboxPair, request *contester_proto.LocalExecutionParameters) (*Sandbox, error) {
	if request.SandboxId != nil {
		return getSandboxById(s, request.GetSandboxId())
	}
	return getSandboxByPath(s, request.GetCurrentDirectory())
}

func fillResult(result *subprocess.SubprocessResult, response *contester_proto.LocalExecutionResult) {
	if result.TotalProcesses > 0 {
		response.TotalProcesses = proto.Uint64(result.TotalProcesses)
	}
	response.ReturnCode = proto.Uint32(result.ExitCode)
	response.Flags = parseSuccessCode(result.SuccessCode)
	response.Time = parseTime(result)
	response.Memory = proto.Uint64(result.PeakMemory)
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
	sub.HardTimeLimit = subprocess.DuFromMicros(request.GetTimeLimitHardMicros())
	sub.MemoryLimit = request.GetMemoryLimit()
	sub.CheckIdleness = request.GetCheckIdleness()
	sub.RestrictUi = request.GetRestrictUi()
	sub.NoJob = request.GetNoJob()

	sub.Environment = fillEnv(request.Environment)

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
	if request.ApplicationName != nil {
		return chmodIfNeeded(*request.ApplicationName, sandbox)
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

type runResult struct {
	second bool
	r      *subprocess.SubprocessResult
	e      error
}

func execAndSend(sub *subprocess.Subprocess, c chan runResult, second bool) {
	var r runResult
	r.second = second
	r.r, r.e = sub.Execute()
	c <- r
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

	err = subprocess.Interconnect(first, second, nil, nil)
	if err != nil {
		return err
	}

	cs := make(chan runResult, 1)
	outstanding := 2

	go execAndSend(first, cs, false)
	go execAndSend(second, cs, true)

	for outstanding > 0 {
		r := <-cs
		outstanding--

		if r.second {
			if r.e != nil {
				err = r.e
			} else {
				response.Second = &contester_proto.LocalExecutionResult{}
				fillResult(r.r, response.Second)
			}
		} else {
			if r.e != nil {
				err = r.e
			} else {
				response.First = &contester_proto.LocalExecutionResult{}
				fillResult(r.r, response.First)
			}
		}
	}

	return err
}

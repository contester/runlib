package service

import (
	"code.google.com/p/goprotobuf/proto"
	"runlib/contester_proto"
	"runlib/subprocess"
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
		result.UserTimeMicros = proto.Uint64(r.UserTime)
	}
	if r.KernelTime != 0 {
		result.KernelTimeMicros = proto.Uint64(r.KernelTime)
	}
	if r.WallTime != 0 {
		result.WallTimeMicros = proto.Uint64(r.WallTime)
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

func (s *Contester) LocalExecute(request *contester_proto.LocalExecutionParameters, response *contester_proto.LocalExecutionResult) error {
	var sandbox *Sandbox
	if request.SandboxId != nil {
		var err error
		sandbox, err = getSandboxById(s.Sandboxes, request.GetSandboxId())
		if err != nil {
			return err
		}
	} else {
		var err error
		sandbox, err = getSandboxByPath(s.Sandboxes, request.GetCurrentDirectory())
		if err != nil {
			return err
		}
	}

	sandbox.Mutex.Lock()
	defer sandbox.Mutex.Unlock()

	sub := subprocess.SubprocessCreate()

	sub.Cmd = &subprocess.CommandLine{
		ApplicationName: request.ApplicationName,
		CommandLine:     request.CommandLine,
		Parameters:      request.CommandLineParameters,
	}

	sub.CurrentDirectory = request.CurrentDirectory

	sub.TimeLimit = request.GetTimeLimitMicros()
	sub.HardTimeLimit = request.GetTimeLimitHardMicros()
	sub.MemoryLimit = request.GetMemoryLimit()
	sub.CheckIdleness = request.GetCheckIdleness()
	sub.RestrictUi = request.GetRestrictUi()
	sub.NoJob = request.GetNoJob()

	sub.Environment = fillEnv(request.Environment)

	sub.StdIn = fillRedirect(request.StdIn)
	sub.StdOut = fillRedirect(request.StdOut)
	sub.StdErr = fillRedirect(request.StdErr)


	if sandbox.Username != nil {
		sub.Login = &subprocess.LoginInfo{
			Username: sandbox.Username,
			Password: sandbox.Password}
	}

	result, err := sub.Execute()

	if err != nil {
		return err
	}

	response.ReturnCode = proto.Uint32(result.ExitCode)
	response.Flags = parseSuccessCode(result.SuccessCode)
	response.Time = parseTime(result)
	response.Memory = proto.Uint64(result.PeakMemory)
	response.StdOut, _ = contester_proto.NewBlob(result.Output)
	response.StdErr, _ = contester_proto.NewBlob(result.Error)

	return nil
}

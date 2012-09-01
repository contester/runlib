package service

import (
	"code.google.com/p/goprotobuf/proto"
	"runlib/contester_proto"
	"runlib/subprocess"
)



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

	sub.Options = &subprocess.PlatformOptions{}

	if sandbox.Login != nil {
		sub.Login = sandbox.Login
		if !sub.NoJob {
			sub.Options.Desktop = s.GData.Desktop.DesktopName
		}
	}

	result, err := sub.Execute()

	if err != nil {
		return err
	}

	if result.TotalProcesses > 0 {
		response.TotalProcesses = proto.Uint64(result.TotalProcesses)
	}
	response.ReturnCode = proto.Uint32(result.ExitCode)
	response.Flags = parseSuccessCode(result.SuccessCode)
	response.Time = parseTime(result)
	response.Memory = proto.Uint64(result.PeakMemory)
	response.StdOut, _ = contester_proto.NewBlob(result.Output)
	response.StdErr, _ = contester_proto.NewBlob(result.Error)

	return nil
}

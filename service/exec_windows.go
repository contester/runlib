package service

import (
	"code.google.com/p/goprotobuf/proto"
	"runlib/contester_proto"
	"runlib/sub32"
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
	if succ&sub32.EF_KILLED != 0 {
		result.Killed = proto.Bool(true)
	}
	if succ&sub32.EF_TIME_LIMIT_HIT != 0 {
		result.TimeLimitHit = proto.Bool(true)
	}
	if succ&sub32.EF_MEMORY_LIMIT_HIT != 0 {
		result.MemoryLimitHit = proto.Bool(true)
	}
	if succ&sub32.EF_INACTIVE != 0 {
		result.Inactive = proto.Bool(true)
	}
	if succ&sub32.EF_TIME_LIMIT_HARD != 0 {
		result.TimeLimitHard = proto.Bool(true)
	}
	if succ&sub32.EF_TIME_LIMIT_HIT_POST != 0 {
		result.TimeLimitHitPost = proto.Bool(true)
	}
	if succ&sub32.EF_MEMORY_LIMIT_HIT_POST != 0 {
		result.MemoryLimitHitPost = proto.Bool(true)
	}
	if succ&sub32.EF_PROCESS_LIMIT_HIT != 0 {
		result.ProcessLimitHit = proto.Bool(true)
	}

	return result
}

func (s *Contester) LocalExecute(request *contester_proto.LocalExecutionParameters, response *contester_proto.LocalExecutionResult) error {
	sub := sub32.SubprocessCreate()

	sub.ApplicationName = request.ApplicationName
	sub.CommandLine = request.CommandLine
	sub.CurrentDirectory = request.CurrentDirectory

	sub.TimeLimit = request.GetTimeLimitMicros()
	sub.HardTimeLimit = request.GetTimeLimitHardMicros()
	sub.MemoryLimit = request.GetMemoryLimit()
	sub.CheckIdleness = request.GetCheckIdleness()
	sub.RestrictUi = request.GetRestrictUi()
	sub.NoJob = request.GetNoJob()

	sub.Environment = fillEnv(request.Environment)

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

	if sandbox.Username != nil {
		sub.Username = sandbox.Username
		sub.Password = sandbox.Password
	}

	sig, err := sub.Start()

	if err != nil {
		return err
	}

	result := <-sig

	response.ReturnCode = proto.Uint32(result.ExitCode)
	response.Flags = parseSuccessCode(result.SuccessCode)

	return nil
}

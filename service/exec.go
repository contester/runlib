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

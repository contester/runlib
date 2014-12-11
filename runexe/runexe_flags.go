package main

import (
	"fmt"
	"strconv"
	"strings"
)

type ProcessAffinityFlag uint64

func (t *ProcessAffinityFlag) String() string {
	return "0" + strconv.FormatUint(uint64(*t), 2)
}

func (t *ProcessAffinityFlag) Set(v string) error {
	if len(v) == 0 {
		return nil
	}

	base := 10
	if v[0] == '0' {
		base = 2
		v = v[1:]
	}

	r, err := strconv.ParseUint(v, base, 64)
	if err != nil {
		return err
	}
	*t = ProcessAffinityFlag(r)
	return nil
}

type TimeLimitFlag uint64

func (t *TimeLimitFlag) String() string {
	return strconv.Itoa(int(*t/1000)) + "ms"
}

func (t *TimeLimitFlag) Set(v string) error {
	v = strings.ToLower(v)
	if strings.HasSuffix(v, "ms") {
		r, err := strconv.Atoi(v[:len(v)-2])
		if err != nil {
			return err
		}
		if r < 0 {
			return fmt.Errorf("Invalid time limit %s", v)
		}
		*t = TimeLimitFlag(r * 1000)
		return nil
	}
	if strings.HasSuffix(v, "s") {
		v = v[:len(v)-1]
	}
	r, err := strconv.ParseFloat(v, 32)
	if err != nil {
		return err
	}
	if r < 0 {
		return fmt.Errorf("Invalid time limit %s", v)
	}
	*t = TimeLimitFlag(r * 1000000)
	return nil
}

type MemoryLimitFlag uint64

func (t *MemoryLimitFlag) String() string {
	return strconv.Itoa(int(*t))
}

func (t *MemoryLimitFlag) Set(v string) error {
	v = strings.ToUpper(v)
	m := 1
	switch v[len(v)-1] {
	case 'M':
		m = 1024 * 1024
	case 'K':
		m = 1024
	case 'G':
		m = 1024 * 1024 * 1024
	}
	if m != 1 {
		v = v[:len(v)-1]
	}
	r, err := strconv.Atoi(v)
	if err != nil {
		return err
	}
	if r < 0 {
		return fmt.Errorf("Invalid memory limit %s", v)
	}
	*t = MemoryLimitFlag(r * m)
	return nil
}

type EnvFlag []string

func (t *EnvFlag) String() string {
	return strings.Join(*t, "|")
}

func (t *EnvFlag) Set(v string) error {
	*t = append(*t, v)
	return nil
}

func PrintUsage() {
	fmt.Printf("runexe 2.0 version %s build %s\n", version, buildid)
	fmt.Println(USAGE)
}

const USAGE = `
This program runs other program(s) for given period of time with specified
restrictions.

Command line format:
  runexe [<global options>] [<process properties>] program [<parameters>]

Global options:
  -help         - show help
  -xml          - print result in xml format (otherwise, use human-readable)
  -show-kernel-mode-time - include kernel-mode time in human-readable format
                  (always included in xml)
  -x            - return process exit code
  -logfile=<f>  - for runexe developers only
  -interactor="<process properties> interactor <parameters>"
                  INTERACTOR MODE
    Launch another process and cross-connect its stdin&stdout with the main
    program. Inside this flag, you can specify any process-controlling flags:
    interactor can have its own limits, credentials, environment, directory.
    In interactor mode, however, -i and -o have no effects on both main
    program and interactor.
  -ri=<f>       - in interactor mode, record program input to file <f>.
  -ro=<f>       - in interactor mode, record program output to file <f>.

Process properties:
  -t <value>    - time limit. Terminate after <value> seconds, you can use
                  suffix ms to switch to milliseconds. Suffix "s" (seconds)
                  can be omitted.
  -m <value>    - memory limit. Terminate if anonymous virtual memory of the
                  process exceeds <value> bytes. Use suffixes K, M, G to
                  specify kilo, mega, gigabytes.
  -D k=v        - environment. If any is specified, existing environment is
                  cleared.
  -d <value>    - current directory for the process.
  -l <value>    - login name. Create process under <value> user.
  -p <value>    - password for user specified in -l. On linux, ignored (but
                  must be present).
  -j <filename> - inject <filename> DLL into process.
  -i <filename> - redirect standard input to <filename>.
  -o <filename> - redirect standard output to <filename>.
  -e <filename> - redirect standard error to <filename>.
  -z            - run process in trusted mode.
  -no-idleness-check - switch off idleness checking.
  -a <value>	- set process affinity to <value>. You can either specify it
                  as plain int, or as a bit mask starting with 0, so 2 and
                  010 are equivalent.
`

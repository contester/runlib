#include <cerrno>
#include <map>

#include <sched.h>
#include <sys/capability.h>
#include <sys/ptrace.h>
#include <sys/types.h>
#include <unistd.h>

#include "clone_helper.h"
#include "mysyscalls.h"

int Exec(const struct CloneParams& params);

int CloneHandler(void * p) {
  const struct CloneParams * const s = reinterpret_cast<CloneParams*>(p);

  return s ? Exec(*s) : -1;
}

// NEWPID NEWNET NEWNS NEWUTS

int HasCapSysAdmin() {
  cap_t caps = cap_get_proc();
  cap_flag_value_t val = CAP_CLEAR;
  cap_get_flag(caps, CAP_SYS_ADMIN, CAP_EFFECTIVE, &val);
  cap_free(caps);
  return val == CAP_SET;
}

pid_t Clone(struct CloneParams * params) {
  int pid = clone(CloneHandler, params->stack,
      SIGCHLD | CLONE_VM | CLONE_SETTLS |
      (HasCapSysAdmin() ? (
          CLONE_NEWPID | CLONE_NEWNET |
          CLONE_NEWNS | CLONE_NEWUTS) : 0),
      reinterpret_cast<void*>(params), 0, params->tls, 0);
  return pid;
}

void Status(int commfd, uint32_t what, uint32_t err) {
  if (commfd == -1)
    return;
  write(commfd, &what, 4);
  write(commfd, &err, 4);
}

int Exec(const struct CloneParams& params) {

  MySyscalls syscalls;

  for (int i = 0; i < 3; ++i) {
      if (params.stdhandles[i] != -1) {
          if (syscalls.dup2(params.stdhandles[i], i) == -1) {
              Status(params.commfd, 16 + i, syscalls.errno_);
              return -1;
          }
         close(params.stdhandles[i]);
      }
  }

  if (params.cwd && (syscalls.chdir(params.cwd) < 0)) {
    Status(params.commfd, 1, syscalls.errno_);
    return -1;
  }

  if (params.suid) {
    if (syscalls.setuid(params.suid) < 0) {
      Status(params.commfd, 2, syscalls.errno_);
      return -1;
    }
  }

  if (syscalls.ptrace(PTRACE_TRACEME, 0, NULL, NULL) == -1) {
    if (syscalls.errno_) {
      Status(params.commfd, 3, syscalls.errno_);
      return -1;
    }
  }

  if (syscalls.execve(params.filename, params.argv, params.envp) == -1) {
    Status(params.commfd, 4, syscalls.errno_);
  }

  return -1;
};

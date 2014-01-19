#pragma once

#include <errno.h>
#include <signal.h>
#include <stdarg.h>
#include <stddef.h>
#include <string.h>
#include <sys/ptrace.h>
#include <sys/resource.h>
#include <sys/time.h>
#include <sys/types.h>
#include <syscall.h>
#include <unistd.h>
#include <linux/unistd.h>
#include <endian.h>

class MySyscalls {
 public:
#define SYS_CPLUSPLUS
#define SYS_ERRNO errno_
#define SYS_INLINE
#undef  SYS_LINUX_SYSCALL_SUPPORT_H
#define SYS_PREFIX -1
#include "linux_syscall_support.h"
  MySyscalls() : errno_(0) {}
  int errno_;
};

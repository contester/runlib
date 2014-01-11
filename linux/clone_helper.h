#pragma once

#include <stdint.h>
#include <sys/types.h>

#ifdef __cplusplus
extern "C" {
#endif

struct CloneParams {
  char *filename;
  char **argv;
  char **envp;
  char *cwd;
  uint32_t suid;
  int32_t stdhandles[3];
  int32_t commfd;

  char *tls;
  char *stack;
};

pid_t Clone(struct CloneParams *params);
int HasCapSysAdmin();

#ifdef __cplusplus
};
#endif

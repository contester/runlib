package service

import (
  "runlib/contester_proto"
)

type Contester struct {}

func (s *Contester) Identify(request *contester_proto.IdentifyRequest, response *contester_proto.IdentifyResponse) error {
  v := "palevo"
  response.InvokerId = &v
  return nil
}
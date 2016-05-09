# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: events.proto

import sys
_b=sys.version_info[0]<3 and (lambda x:x) or (lambda x:x.encode('latin1'))
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from google.protobuf import reflection as _reflection
from google.protobuf import symbol_database as _symbol_database
from google.protobuf import descriptor_pb2
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


import fabric_pb2 as fabric__pb2


DESCRIPTOR = _descriptor.FileDescriptor(
  name='events.proto',
  package='protos',
  syntax='proto3',
  serialized_pb=_b('\n\x0c\x65vents.proto\x12\x06protos\x1a\x0c\x66\x61\x62ric.proto\"\x88\x01\n\x08Interest\x12\x11\n\teventType\x18\x01 \x01(\t\x12\x33\n\x0cresponseType\x18\x02 \x01(\x0e\x32\x1d.protos.Interest.ResponseType\"4\n\x0cResponseType\x12\x0c\n\x08\x44ONTSEND\x10\x00\x12\x0c\n\x08PROTOBUF\x10\x01\x12\x08\n\x04JSON\x10\x02\",\n\x08Register\x12 \n\x06\x65vents\x18\x01 \x03(\x0b\x32\x10.protos.Interest\"-\n\x07Generic\x12\x11\n\teventType\x18\x01 \x01(\t\x12\x0f\n\x07payload\x18\x02 \x01(\x0c\"z\n\x05\x45vent\x12$\n\x08register\x18\x01 \x01(\x0b\x32\x10.protos.RegisterH\x00\x12\x1e\n\x05\x62lock\x18\x02 \x01(\x0b\x32\r.protos.BlockH\x00\x12\"\n\x07generic\x18\x03 \x01(\x0b\x32\x0f.protos.GenericH\x00\x42\x07\n\x05\x45vent24\n\x06\x45vents\x12*\n\x04\x43hat\x12\r.protos.Event\x1a\r.protos.Event\"\x00(\x01\x30\x01\x62\x06proto3')
  ,
  dependencies=[fabric__pb2.DESCRIPTOR,])
_sym_db.RegisterFileDescriptor(DESCRIPTOR)



_INTEREST_RESPONSETYPE = _descriptor.EnumDescriptor(
  name='ResponseType',
  full_name='protos.Interest.ResponseType',
  filename=None,
  file=DESCRIPTOR,
  values=[
    _descriptor.EnumValueDescriptor(
      name='DONTSEND', index=0, number=0,
      options=None,
      type=None),
    _descriptor.EnumValueDescriptor(
      name='PROTOBUF', index=1, number=1,
      options=None,
      type=None),
    _descriptor.EnumValueDescriptor(
      name='JSON', index=2, number=2,
      options=None,
      type=None),
  ],
  containing_type=None,
  options=None,
  serialized_start=123,
  serialized_end=175,
)
_sym_db.RegisterEnumDescriptor(_INTEREST_RESPONSETYPE)


_INTEREST = _descriptor.Descriptor(
  name='Interest',
  full_name='protos.Interest',
  filename=None,
  file=DESCRIPTOR,
  containing_type=None,
  fields=[
    _descriptor.FieldDescriptor(
      name='eventType', full_name='protos.Interest.eventType', index=0,
      number=1, type=9, cpp_type=9, label=1,
      has_default_value=False, default_value=_b("").decode('utf-8'),
      message_type=None, enum_type=None, containing_type=None,
      is_extension=False, extension_scope=None,
      options=None),
    _descriptor.FieldDescriptor(
      name='responseType', full_name='protos.Interest.responseType', index=1,
      number=2, type=14, cpp_type=8, label=1,
      has_default_value=False, default_value=0,
      message_type=None, enum_type=None, containing_type=None,
      is_extension=False, extension_scope=None,
      options=None),
  ],
  extensions=[
  ],
  nested_types=[],
  enum_types=[
    _INTEREST_RESPONSETYPE,
  ],
  options=None,
  is_extendable=False,
  syntax='proto3',
  extension_ranges=[],
  oneofs=[
  ],
  serialized_start=39,
  serialized_end=175,
)


_REGISTER = _descriptor.Descriptor(
  name='Register',
  full_name='protos.Register',
  filename=None,
  file=DESCRIPTOR,
  containing_type=None,
  fields=[
    _descriptor.FieldDescriptor(
      name='events', full_name='protos.Register.events', index=0,
      number=1, type=11, cpp_type=10, label=3,
      has_default_value=False, default_value=[],
      message_type=None, enum_type=None, containing_type=None,
      is_extension=False, extension_scope=None,
      options=None),
  ],
  extensions=[
  ],
  nested_types=[],
  enum_types=[
  ],
  options=None,
  is_extendable=False,
  syntax='proto3',
  extension_ranges=[],
  oneofs=[
  ],
  serialized_start=177,
  serialized_end=221,
)


_GENERIC = _descriptor.Descriptor(
  name='Generic',
  full_name='protos.Generic',
  filename=None,
  file=DESCRIPTOR,
  containing_type=None,
  fields=[
    _descriptor.FieldDescriptor(
      name='eventType', full_name='protos.Generic.eventType', index=0,
      number=1, type=9, cpp_type=9, label=1,
      has_default_value=False, default_value=_b("").decode('utf-8'),
      message_type=None, enum_type=None, containing_type=None,
      is_extension=False, extension_scope=None,
      options=None),
    _descriptor.FieldDescriptor(
      name='payload', full_name='protos.Generic.payload', index=1,
      number=2, type=12, cpp_type=9, label=1,
      has_default_value=False, default_value=_b(""),
      message_type=None, enum_type=None, containing_type=None,
      is_extension=False, extension_scope=None,
      options=None),
  ],
  extensions=[
  ],
  nested_types=[],
  enum_types=[
  ],
  options=None,
  is_extendable=False,
  syntax='proto3',
  extension_ranges=[],
  oneofs=[
  ],
  serialized_start=223,
  serialized_end=268,
)


_EVENT = _descriptor.Descriptor(
  name='Event',
  full_name='protos.Event',
  filename=None,
  file=DESCRIPTOR,
  containing_type=None,
  fields=[
    _descriptor.FieldDescriptor(
      name='register', full_name='protos.Event.register', index=0,
      number=1, type=11, cpp_type=10, label=1,
      has_default_value=False, default_value=None,
      message_type=None, enum_type=None, containing_type=None,
      is_extension=False, extension_scope=None,
      options=None),
    _descriptor.FieldDescriptor(
      name='block', full_name='protos.Event.block', index=1,
      number=2, type=11, cpp_type=10, label=1,
      has_default_value=False, default_value=None,
      message_type=None, enum_type=None, containing_type=None,
      is_extension=False, extension_scope=None,
      options=None),
    _descriptor.FieldDescriptor(
      name='generic', full_name='protos.Event.generic', index=2,
      number=3, type=11, cpp_type=10, label=1,
      has_default_value=False, default_value=None,
      message_type=None, enum_type=None, containing_type=None,
      is_extension=False, extension_scope=None,
      options=None),
  ],
  extensions=[
  ],
  nested_types=[],
  enum_types=[
  ],
  options=None,
  is_extendable=False,
  syntax='proto3',
  extension_ranges=[],
  oneofs=[
    _descriptor.OneofDescriptor(
      name='Event', full_name='protos.Event.Event',
      index=0, containing_type=None, fields=[]),
  ],
  serialized_start=270,
  serialized_end=392,
)

_INTEREST.fields_by_name['responseType'].enum_type = _INTEREST_RESPONSETYPE
_INTEREST_RESPONSETYPE.containing_type = _INTEREST
_REGISTER.fields_by_name['events'].message_type = _INTEREST
_EVENT.fields_by_name['register'].message_type = _REGISTER
_EVENT.fields_by_name['block'].message_type = fabric__pb2._BLOCK
_EVENT.fields_by_name['generic'].message_type = _GENERIC
_EVENT.oneofs_by_name['Event'].fields.append(
  _EVENT.fields_by_name['register'])
_EVENT.fields_by_name['register'].containing_oneof = _EVENT.oneofs_by_name['Event']
_EVENT.oneofs_by_name['Event'].fields.append(
  _EVENT.fields_by_name['block'])
_EVENT.fields_by_name['block'].containing_oneof = _EVENT.oneofs_by_name['Event']
_EVENT.oneofs_by_name['Event'].fields.append(
  _EVENT.fields_by_name['generic'])
_EVENT.fields_by_name['generic'].containing_oneof = _EVENT.oneofs_by_name['Event']
DESCRIPTOR.message_types_by_name['Interest'] = _INTEREST
DESCRIPTOR.message_types_by_name['Register'] = _REGISTER
DESCRIPTOR.message_types_by_name['Generic'] = _GENERIC
DESCRIPTOR.message_types_by_name['Event'] = _EVENT

Interest = _reflection.GeneratedProtocolMessageType('Interest', (_message.Message,), dict(
  DESCRIPTOR = _INTEREST,
  __module__ = 'events_pb2'
  # @@protoc_insertion_point(class_scope:protos.Interest)
  ))
_sym_db.RegisterMessage(Interest)

Register = _reflection.GeneratedProtocolMessageType('Register', (_message.Message,), dict(
  DESCRIPTOR = _REGISTER,
  __module__ = 'events_pb2'
  # @@protoc_insertion_point(class_scope:protos.Register)
  ))
_sym_db.RegisterMessage(Register)

Generic = _reflection.GeneratedProtocolMessageType('Generic', (_message.Message,), dict(
  DESCRIPTOR = _GENERIC,
  __module__ = 'events_pb2'
  # @@protoc_insertion_point(class_scope:protos.Generic)
  ))
_sym_db.RegisterMessage(Generic)

Event = _reflection.GeneratedProtocolMessageType('Event', (_message.Message,), dict(
  DESCRIPTOR = _EVENT,
  __module__ = 'events_pb2'
  # @@protoc_insertion_point(class_scope:protos.Event)
  ))
_sym_db.RegisterMessage(Event)


import abc
import six
from grpc.beta import implementations as beta_implementations
from grpc.framework.common import cardinality
from grpc.framework.interfaces.face import utilities as face_utilities

class BetaEventsServicer(six.with_metaclass(abc.ABCMeta, object)):
  """<fill me in later!>"""
  @abc.abstractmethod
  def Chat(self, request_iterator, context):
    raise NotImplementedError()

class BetaEventsStub(six.with_metaclass(abc.ABCMeta, object)):
  """The interface to which stubs will conform."""
  @abc.abstractmethod
  def Chat(self, request_iterator, timeout):
    raise NotImplementedError()

def beta_create_Events_server(servicer, pool=None, pool_size=None, default_timeout=None, maximum_timeout=None):
  import events_pb2
  import events_pb2
  request_deserializers = {
    ('protos.Events', 'Chat'): events_pb2.Event.FromString,
  }
  response_serializers = {
    ('protos.Events', 'Chat'): events_pb2.Event.SerializeToString,
  }
  method_implementations = {
    ('protos.Events', 'Chat'): face_utilities.stream_stream_inline(servicer.Chat),
  }
  server_options = beta_implementations.server_options(request_deserializers=request_deserializers, response_serializers=response_serializers, thread_pool=pool, thread_pool_size=pool_size, default_timeout=default_timeout, maximum_timeout=maximum_timeout)
  return beta_implementations.server(method_implementations, options=server_options)

def beta_create_Events_stub(channel, host=None, metadata_transformer=None, pool=None, pool_size=None):
  import events_pb2
  import events_pb2
  request_serializers = {
    ('protos.Events', 'Chat'): events_pb2.Event.SerializeToString,
  }
  response_deserializers = {
    ('protos.Events', 'Chat'): events_pb2.Event.FromString,
  }
  cardinalities = {
    'Chat': cardinality.Cardinality.STREAM_STREAM,
  }
  stub_options = beta_implementations.stub_options(host=host, metadata_transformer=metadata_transformer, request_serializers=request_serializers, response_deserializers=response_deserializers, thread_pool=pool, thread_pool_size=pool_size)
  return beta_implementations.dynamic_stub(channel, 'protos.Events', cardinalities, options=stub_options)
# @@protoc_insertion_point(module_scope)

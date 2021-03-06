// Generated by the protocol buffer compiler.  DO NOT EDIT!
// source: EventChat.proto

package com.example.authclientproto;

public final class ClientProto {
  private ClientProto() {}
  public static void registerAllExtensions(
      com.google.protobuf.ExtensionRegistryLite registry) {
  }

  public static void registerAllExtensions(
      com.google.protobuf.ExtensionRegistry registry) {
    registerAllExtensions(
        (com.google.protobuf.ExtensionRegistryLite) registry);
  }
  static final com.google.protobuf.Descriptors.Descriptor
    internal_static_pb_Chat_descriptor;
  static final 
    com.google.protobuf.GeneratedMessageV3.FieldAccessorTable
      internal_static_pb_Chat_fieldAccessorTable;
  static final com.google.protobuf.Descriptors.Descriptor
    internal_static_pb_Chat_MoreEntry_descriptor;
  static final 
    com.google.protobuf.GeneratedMessageV3.FieldAccessorTable
      internal_static_pb_Chat_MoreEntry_fieldAccessorTable;
  static final com.google.protobuf.Descriptors.Descriptor
    internal_static_pb_Event_descriptor;
  static final 
    com.google.protobuf.GeneratedMessageV3.FieldAccessorTable
      internal_static_pb_Event_fieldAccessorTable;
  static final com.google.protobuf.Descriptors.Descriptor
    internal_static_pb_Event_MoreEntry_descriptor;
  static final 
    com.google.protobuf.GeneratedMessageV3.FieldAccessorTable
      internal_static_pb_Event_MoreEntry_fieldAccessorTable;
  static final com.google.protobuf.Descriptors.Descriptor
    internal_static_pb_OnlineMeet_descriptor;
  static final 
    com.google.protobuf.GeneratedMessageV3.FieldAccessorTable
      internal_static_pb_OnlineMeet_fieldAccessorTable;
  static final com.google.protobuf.Descriptors.Descriptor
    internal_static_pb_OnlineMeet_SupportLinksEntry_descriptor;
  static final 
    com.google.protobuf.GeneratedMessageV3.FieldAccessorTable
      internal_static_pb_OnlineMeet_SupportLinksEntry_fieldAccessorTable;
  static final com.google.protobuf.Descriptors.Descriptor
    internal_static_pb_PhysicalMeet_descriptor;
  static final 
    com.google.protobuf.GeneratedMessageV3.FieldAccessorTable
      internal_static_pb_PhysicalMeet_fieldAccessorTable;

  public static com.google.protobuf.Descriptors.FileDescriptor
      getDescriptor() {
    return descriptor;
  }
  private static  com.google.protobuf.Descriptors.FileDescriptor
      descriptor;
  static {
    java.lang.String[] descriptorData = {
      "\n\017EventChat.proto\022\002pb\032\030protobuf/timestam" +
      "p.proto\"\213\002\n\004Chat\022\014\n\004Name\030\001 \001(\t\022\016\n\006IsOpen" +
      "\030\002 \001(\010\022\023\n\013Description\030\003 \001(\t\022\014\n\004Tags\030\004 \003(" +
      "\t\022\020\n\010Resource\030\005 \001(\t\022 \n\004More\030\006 \003(\0132\022.pb.C" +
      "hat.MoreEntry\022\017\n\007Members\030\010 \003(\t\022,\n\010Creati" +
      "on\030\t \001(\0132\032.google.protobuf.Timestamp\022\017\n\007" +
      "Creator\030\n \001(\t\022\021\n\tFromEvent\030\013 \001(\t\032+\n\tMore" +
      "Entry\022\013\n\003key\030\001 \001(\t\022\r\n\005value\030\002 \001(\t:\0028\001\"\314\002" +
      "\n\005Event\022\014\n\004Name\030\001 \001(\t\022\016\n\006IsOpen\030\002 \001(\010\022\023\n" +
      "\013Description\030\003 \001(\t\022\014\n\004Tags\030\004 \003(\t\022\020\n\010Reso" +
      "urce\030\005 \001(\t\022!\n\004More\030\006 \003(\0132\023.pb.Event.More" +
      "Entry\022\017\n\007Members\030\010 \003(\t\022,\n\010Creation\030\t \001(\013" +
      "2\032.google.protobuf.Timestamp\022\017\n\007Creator\030" +
      "\n \001(\t\022\014\n\004Chat\030\014 \001(\t\022\"\n\010Physical\030\024 \001(\0132\020." +
      "pb.PhysicalMeet\022\036\n\006Online\030\036 \001(\0132\016.pb.Onl" +
      "ineMeet\032+\n\tMoreEntry\022\013\n\003key\030\001 \001(\t\022\r\n\005val" +
      "ue\030\002 \001(\t:\0028\001\"\207\001\n\nOnlineMeet\022\014\n\004Type\030\001 \001(" +
      "\t\0226\n\014SupportLinks\030\002 \003(\0132 .pb.OnlineMeet." +
      "SupportLinksEntry\0323\n\021SupportLinksEntry\022\013" +
      "\n\003key\030\001 \001(\t\022\r\n\005value\030\002 \001(\t:\0028\001\"W\n\014Physic" +
      "alMeet\022\014\n\004Type\030\001 \001(\t\022\014\n\004Maps\030\002 \001(\t\022\026\n\016Lo" +
      "cDescription\030\003 \001(\t\022\023\n\013WhatToBring\030\004 \003(\tB" +
      "2\n\033com.example.authclientprotoB\013ClientPr" +
      "otoP\001Z\004.;pbb\006proto3"
    };
    descriptor = com.google.protobuf.Descriptors.FileDescriptor
      .internalBuildGeneratedFileFrom(descriptorData,
        new com.google.protobuf.Descriptors.FileDescriptor[] {
          com.google.protobuf.TimestampProto.getDescriptor(),
        });
    internal_static_pb_Chat_descriptor =
      getDescriptor().getMessageTypes().get(0);
    internal_static_pb_Chat_fieldAccessorTable = new
      com.google.protobuf.GeneratedMessageV3.FieldAccessorTable(
        internal_static_pb_Chat_descriptor,
        new java.lang.String[] { "Name", "IsOpen", "Description", "Tags", "Resource", "More", "Members", "Creation", "Creator", "FromEvent", });
    internal_static_pb_Chat_MoreEntry_descriptor =
      internal_static_pb_Chat_descriptor.getNestedTypes().get(0);
    internal_static_pb_Chat_MoreEntry_fieldAccessorTable = new
      com.google.protobuf.GeneratedMessageV3.FieldAccessorTable(
        internal_static_pb_Chat_MoreEntry_descriptor,
        new java.lang.String[] { "Key", "Value", });
    internal_static_pb_Event_descriptor =
      getDescriptor().getMessageTypes().get(1);
    internal_static_pb_Event_fieldAccessorTable = new
      com.google.protobuf.GeneratedMessageV3.FieldAccessorTable(
        internal_static_pb_Event_descriptor,
        new java.lang.String[] { "Name", "IsOpen", "Description", "Tags", "Resource", "More", "Members", "Creation", "Creator", "Chat", "Physical", "Online", });
    internal_static_pb_Event_MoreEntry_descriptor =
      internal_static_pb_Event_descriptor.getNestedTypes().get(0);
    internal_static_pb_Event_MoreEntry_fieldAccessorTable = new
      com.google.protobuf.GeneratedMessageV3.FieldAccessorTable(
        internal_static_pb_Event_MoreEntry_descriptor,
        new java.lang.String[] { "Key", "Value", });
    internal_static_pb_OnlineMeet_descriptor =
      getDescriptor().getMessageTypes().get(2);
    internal_static_pb_OnlineMeet_fieldAccessorTable = new
      com.google.protobuf.GeneratedMessageV3.FieldAccessorTable(
        internal_static_pb_OnlineMeet_descriptor,
        new java.lang.String[] { "Type", "SupportLinks", });
    internal_static_pb_OnlineMeet_SupportLinksEntry_descriptor =
      internal_static_pb_OnlineMeet_descriptor.getNestedTypes().get(0);
    internal_static_pb_OnlineMeet_SupportLinksEntry_fieldAccessorTable = new
      com.google.protobuf.GeneratedMessageV3.FieldAccessorTable(
        internal_static_pb_OnlineMeet_SupportLinksEntry_descriptor,
        new java.lang.String[] { "Key", "Value", });
    internal_static_pb_PhysicalMeet_descriptor =
      getDescriptor().getMessageTypes().get(3);
    internal_static_pb_PhysicalMeet_fieldAccessorTable = new
      com.google.protobuf.GeneratedMessageV3.FieldAccessorTable(
        internal_static_pb_PhysicalMeet_descriptor,
        new java.lang.String[] { "Type", "Maps", "LocDescription", "WhatToBring", });
    com.google.protobuf.TimestampProto.getDescriptor();
  }

  // @@protoc_insertion_point(outer_class_scope)
}

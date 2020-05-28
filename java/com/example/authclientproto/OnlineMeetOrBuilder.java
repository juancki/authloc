// Generated by the protocol buffer compiler.  DO NOT EDIT!
// source: EventChat.proto

package com.example.authclientproto;

public interface OnlineMeetOrBuilder extends
    // @@protoc_insertion_point(interface_extends:pb.OnlineMeet)
    com.google.protobuf.MessageOrBuilder {

  /**
   * <code>string Type = 1;</code>
   * @return The type.
   */
  java.lang.String getType();
  /**
   * <code>string Type = 1;</code>
   * @return The bytes for type.
   */
  com.google.protobuf.ByteString
      getTypeBytes();

  /**
   * <pre>
   * instagram,discord, movie link
   * </pre>
   *
   * <code>map&lt;string, string&gt; SupportLinks = 2;</code>
   */
  int getSupportLinksCount();
  /**
   * <pre>
   * instagram,discord, movie link
   * </pre>
   *
   * <code>map&lt;string, string&gt; SupportLinks = 2;</code>
   */
  boolean containsSupportLinks(
      java.lang.String key);
  /**
   * Use {@link #getSupportLinksMap()} instead.
   */
  @java.lang.Deprecated
  java.util.Map<java.lang.String, java.lang.String>
  getSupportLinks();
  /**
   * <pre>
   * instagram,discord, movie link
   * </pre>
   *
   * <code>map&lt;string, string&gt; SupportLinks = 2;</code>
   */
  java.util.Map<java.lang.String, java.lang.String>
  getSupportLinksMap();
  /**
   * <pre>
   * instagram,discord, movie link
   * </pre>
   *
   * <code>map&lt;string, string&gt; SupportLinks = 2;</code>
   */

  java.lang.String getSupportLinksOrDefault(
      java.lang.String key,
      java.lang.String defaultValue);
  /**
   * <pre>
   * instagram,discord, movie link
   * </pre>
   *
   * <code>map&lt;string, string&gt; SupportLinks = 2;</code>
   */

  java.lang.String getSupportLinksOrThrow(
      java.lang.String key);
}

using Go = import "/go.capnp";

$Go.package("internals");
$Go.import("github.com/finogeeks/ligase/plugins/message/internals");

@0xc5519b9e71d66d17;

struct DeviceCapn {
    id           @0 :Text;
    userID       @1 :Text;
    displayName  @2 :Text;
    deviceType   @3 :Text;
    isHuman      @4 :Bool;
    identifier   @5 :Text;
    createTs     @6 :Int64;
    lastActiveTs @7 :Int64;
}

struct InputMsgCapn {
    msgType         @0 :Int32;
    device          @1 :DeviceCapn;
    payload         @2 :Data;
    reply           @3 :Int64;
}

struct OutputMsgCapn {
    msgType     @0 :Int32;
    code        @1 :Int64;
    headers     @2 :Data;
    body        @3 :Data;
    reply       @5 :Int64;
}
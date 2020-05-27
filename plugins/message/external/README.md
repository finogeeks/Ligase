capn 使用：
1.根据消息类型编写对应的 capn schema 定义， 比如 login.capn。
2.根据 capn schema 定义， 比如 login.capn，定义编译生成 go 代码，login.capn.go：
    VENDER_PATH 为 dendrite vender 目录。
    本机安装 capnp 程序: brew install capnp。
    编译需要使用 go 插件, 已生成在：export PATH=$PATH:$VENDER_PATH/bin
    编译：capnp compile -I$VENDER_PATH/src/zombiezen.com/go/capnproto2/std -ogo login.capn
3.使用生成的 go 代码编写 encode 和 decode 过程。

编译和调用 go 代码都需要依赖 zombiezen.com/go/capnproto2。
调用过程不需要依赖本地 capnp 库。
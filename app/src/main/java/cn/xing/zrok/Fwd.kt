package cn.xing.zrok
object Fwd {
    init { System.loadLibrary("fwd") }
    external fun nativeStart(port: Int, sockName: String): Boolean
    external fun nativeStop()
}

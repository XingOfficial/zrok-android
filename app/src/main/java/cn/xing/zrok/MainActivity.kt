package cn.xing.zrok
import android.content.Intent
import android.net.Uri
import android.os.Environment
import android.provider.Settings
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.*
import org.json.JSONArray
import org.json.JSONObject
data class ShareRow(val envZId: String, val shareToken: String, val url: String, val desc: String)
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        startForegroundService(Intent(this, TunnelService::class.java))
        val rootDir = filesDir.absolutePath + "/zrok2"
        setContent { App(rootDir) }
    }
}
@Composable
fun App(rootDir: String) {
    var screen by remember { mutableStateOf("login") }
    var token by remember { mutableStateOf("") }
    var log by remember { mutableStateOf("zrok 客户端") }
    MaterialTheme {
        Surface(Modifier.fillMaxSize()) {
            Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp)) {
                if (screen == "login") {
                    LoginScreen(rootDir, onToken = { token = it }, onDone = { msg ->
                        log = msg
                        if (msg.contains("\"ok\":true")) screen = "main"
                    })
                } else {
                    MainScreen(rootDir, token, onLog = { log = it })
                }
                Spacer(Modifier.height(12.dp))
                Text(log, style = MaterialTheme.typography.bodySmall)
            }
        }
    }
}
@Composable
fun LoginScreen(rootDir: String, onToken: (String) -> Unit, onDone: (String) -> Unit) {
    var token by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    Text("zrok 登录", style = MaterialTheme.typography.headlineSmall)
    Spacer(Modifier.height(12.dp))
    OutlinedTextField(value = token, onValueChange = {
        token = it; onToken(it)
    }, label = { Text("Account Token") }, modifier = Modifier.fillMaxWidth())
    Spacer(Modifier.height(12.dp))
    Button(enabled = !busy, onClick = {
        if (!Environment.isExternalStorageManager()) {
            try {
                startActivity(Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION,
                    Uri.parse("package:cn.xing.zrok")))
            } catch (e: Exception) {
                startActivity(Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION))
            }
        } else {
            busy = true
            onDone(zrokcore.Zrokcore.enable(rootDir, token, "android-app"))
            busy = false
        }
    }) { Text(if (busy) "启用中…" else "启用环境") }
}
@Composable
fun MainScreen(rootDir: String, token: String, onLog: (String) -> Unit) {
    var shares by remember { mutableStateOf(listOf<ShareRow>()) }
    var tunnels by remember { mutableStateOf(listOf<JSONObject>()) }
    var dlgFor by remember { mutableStateOf<String?>(null) }
    var accToken by remember { mutableStateOf("") }
    var accPort by remember { mutableStateOf("9443") }
    fun refresh() {
        val ov = JSONObject(zrokcore.Zrokcore.overview(token))
        if (ov.optBoolean("ok")) {
            val rows = mutableListOf<ShareRow>()
            val envs = ov.optJSONObject("data")?.optJSONArray("environments") ?: JSONArray()
            for (i in 0 until envs.length()) {
                val e = envs.getJSONObject(i)
                val env = e.getJSONObject("environment")
                val ss = e.optJSONArray("shares") ?: JSONArray()
                for (j in 0 until ss.length()) {
                    val s = ss.getJSONObject(j)
                    rows.add(ShareRow(env.optString("zId"), s.optString("shareToken"),
                        s.optString("frontendEndpoint"), s.optString("backendProxyEndpoint")))
                }
            }
            shares = rows
        } else onLog(ov.toString())
    }
    LaunchedEffect(Unit) {
        while (true) {
            val st = JSONObject(zrokcore.Zrokcore.stats())
            if (st.optBoolean("ok")) {
                val arr = st.optJSONArray("data") ?: JSONArray()
                tunnels = (0 until arr.length()).map { arr.getJSONObject(it) }
            }
            delay(1000)
        }
    }
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
        Text("总览", style = MaterialTheme.typography.headlineSmall)
        Button(onClick = { refresh(); onLog("已刷新") }) { Text("刷新") }
    }
    Spacer(Modifier.height(8.dp))
    if (shares.isEmpty()) Text("还没有 share，点下面新建，或先刷新")
    shares.forEach { s ->
        Card(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
            Column(Modifier.padding(10.dp)) {
                Text(s.url, style = MaterialTheme.typography.titleMedium)
                Text(s.desc, style = MaterialTheme.typography.bodySmall)
                Row {
                    TextButton(onClick = {
                        onLog(zrokcore.Zrokcore.startHost(rootDir, s.shareToken, s.desc))
                    }) { Text("开隧道") }
                    TextButton(onClick = {
                        onLog(zrokcore.Zrokcore.deleteShare(token, s.envZId, s.shareToken))
                        refresh()
                    }) { Text("删除", color = MaterialTheme.colorScheme.error) }
                }
            }
        }
    }
    Spacer(Modifier.height(8.dp))
    Row {
        Button(onClick = { dlgFor = shares.firstOrNull()?.envZId ?: "" }) { Text("+ 新建 share") }
        Spacer(Modifier.width(8.dp))
        OutlinedTextField(value = accToken, onValueChange = { accToken = it },
            label = { Text("私有 share token") }, modifier = Modifier.weight(1f))
    }
    Spacer(Modifier.height(4.dp))
    Row(verticalAlignment = androidx.compose.ui.Alignment.CenterVertically) {
        OutlinedTextField(value = accPort, onValueChange = { accPort = it }, label = { Text("本地端口") },
            modifier = Modifier.width(120.dp))
        Spacer(Modifier.width(8.dp))
        Button(onClick = {
            val sock = rootDir + "/acc-" + accToken
            Fwd.nativeStart(accPort.toInt(), sock)
            onLog(zrokcore.Zrokcore.startAccess(rootDir, accToken, sock, accPort.toLong()))
        }) { Text("接入") }
    }
    Spacer(Modifier.height(12.dp))
    Text("隧道监控", style = MaterialTheme.typography.headlineSmall)
    tunnels.forEach { t ->
        Card(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
            Column(Modifier.padding(10.dp)) {
                Text("#${t.optString("id")} ${t.optString("mode")} ${t.optString("state")}")
                Text("${t.optString("shareToken")} → ${t.optString("target")}")
                Text("↑${t.optLong("tx") / 1024}KB ↓${t.optLong("rx") / 1024}KB 连接:${t.optInt("conns")}")
                if (t.optString("lastError").isNotEmpty()) Text(t.optString("lastError"),
                    color = MaterialTheme.colorScheme.error)
                TextButton(onClick = { onLog(zrokcore.Zrokcore.stop(t.optString("id"))) }) { Text("停止") }
            }
        }
    }
    dlgFor?.let { envZId ->
        var target by remember { mutableStateOf("http://localhost:8080") }
        var uname by remember { mutableStateOf("") }
        AlertDialog(onDismissRequest = {}, title = { Text("新建 share") },
            text = {
                Column {
                    OutlinedTextField(value = target, onValueChange = { target = it },
                        label = { Text("目标 如 localhost:8080") })
                    OutlinedTextField(value = uname, onValueChange = { uname = it },
                        label = { Text("唯一名 可选") })
                }
            },
            confirmButton = {
                TextButton(onClick = {
                    onLog(zrokcore.Zrokcore.createShare(token, envZId, target.trim(), uname.trim()))
                    dlgFor = null; refresh()
                }) { Text("创建") }
            },
            dismissButton = { TextButton(onClick = { dlgFor = null }) { Text("取消") } })
    }
}

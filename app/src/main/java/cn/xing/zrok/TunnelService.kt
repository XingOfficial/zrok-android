package cn.xing.zrok
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.IBinder
class TunnelService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val ch = NotificationChannel("zrok", "zrok 隧道", NotificationManager.IMPORTANCE_LOW)
        (getSystemService(NOTIFICATION_SERVICE) as NotificationManager).createNotificationChannel(ch)
        val n: Notification = Notification.Builder(this, "zrok")
            .setContentTitle("zrok 隧道运行中")
            .setSmallIcon(android.R.drawable.stat_notify_sync)
            .build()
        startForeground(1, n)
        return START_STICKY
    }
}

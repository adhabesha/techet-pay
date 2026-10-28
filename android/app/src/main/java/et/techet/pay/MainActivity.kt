package et.techet.pay

import android.graphics.Color as AndroidColor
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.DisposableEffect
import androidx.lifecycle.viewmodel.compose.viewModel
import et.techet.pay.ui.GlassBackground
import et.techet.pay.ui.Root
import et.techet.pay.ui.TechETTheme
import et.techet.pay.ui.ThemeState
import androidx.compose.foundation.isSystemInDarkTheme

class MainActivity : ComponentActivity() {
    private var systemChecked = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.light(AndroidColor.TRANSPARENT, AndroidColor.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.light(AndroidColor.TRANSPARENT, AndroidColor.TRANSPARENT)
        )
        setContent {
            val vm: AppViewModel = viewModel()
            val prefs = getSharedPreferences("techet", MODE_PRIVATE)
            if (!prefs.contains("dark") && !systemChecked) {
                systemChecked = true
                ThemeState.dark = isSystemInDarkTheme()
            }
            val dark = ThemeState.dark
            DisposableEffect(dark) {
                val bar = if (dark) SystemBarStyle.dark(AndroidColor.TRANSPARENT)
                else SystemBarStyle.light(AndroidColor.TRANSPARENT, AndroidColor.TRANSPARENT)
                enableEdgeToEdge(statusBarStyle = bar, navigationBarStyle = bar)
                onDispose { }
            }
            TechETTheme {
                GlassBackground {
                    Root(vm)
                }
            }
        }
    }
}

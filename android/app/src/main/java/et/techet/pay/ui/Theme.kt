package et.techet.pay.ui

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color

object ThemeState {
    /** Backed by Compose state, so every screen redraws when it changes. */
    var dark by mutableStateOf(false)
}

object TP {
    // Dark surfaces used for panels, buttons and the nav pill (same in both modes)
    val Navy900 = Color(0xFF111312)
    val Navy800 = Color(0xFF222629)
    val Navy700 = Color(0xFF2B3136)
    val Blue600 = Color(0xFF5FA800)
    val Blue500 = Color(0xFF75C400)
    val Blue400 = Color(0xFF91DA33)
    val Blue300 = Color(0xFFB5E86B)
    val Blue200 = Color(0xFFD9F2AE)
    val Blue100 = Color(0xFFEEF8DA)
    val Sky = Color(0xFFBDE485)
    val Success = Color(0xFF1FB67A)
    val Danger = Color(0xFFE5484D)
    val Warning = Color(0xFFF5A524)

    // Theme-aware tokens
    val Ice: Color get() = if (ThemeState.dark) Color(0xFF0C0D0C) else Color(0xFFEFF0EE)
    val Ink: Color get() = if (ThemeState.dark) Color(0xFFF2F4F2) else Color(0xFF111312)
    val Muted: Color get() = if (ThemeState.dark) Color(0xFF9AA39B) else Color(0xFF6B746C)
    val Line: Color get() = if (ThemeState.dark) Color(0xFF2A2E2A) else Color(0xFFDDE2DC)
    val Card: Color get() = if (ThemeState.dark) Color(0xFF181A18) else Color(0xFFFFFFFF)
}

/** Card surface: white in light mode, near-black in dark mode. */
val GlassFill: Brush get() = Brush.linearGradient(listOf(TP.Card, TP.Card))

/** No sheen any more: the new style is flat and bold. */
val GlassSheen: Brush get() = Brush.verticalGradient(listOf(Color.Transparent, Color.Transparent))

/** Hairline edge around cards. */
val GlassBorder: Brush get() = Brush.linearGradient(listOf(TP.Line, TP.Line))

/** The floating navigation pill is black in both modes. */
val NavFill: Brush get() = Brush.verticalGradient(listOf(Color(0xFF111312), Color(0xFF111312)))

val ButtonBrush: Brush = Brush.linearGradient(listOf(TP.Blue400, TP.Blue500))

val ButtonSheen: Brush = Brush.verticalGradient(listOf(Color.Transparent, Color.Transparent))

@Composable
fun TechETTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = (if (ThemeState.dark) darkColorScheme() else lightColorScheme()).copy(
            primary = TP.Blue500,
            onPrimary = TP.Navy800,
            secondary = TP.Blue500,
            background = TP.Ice,
            surface = TP.Ice,
            onBackground = TP.Ink,
            onSurface = TP.Ink,
            error = TP.Danger
        ),
        content = content
    )
}

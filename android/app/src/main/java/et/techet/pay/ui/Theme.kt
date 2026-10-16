package et.techet.pay.ui

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color

object TP {
    val Navy900 = Color(0xFF0A1740)
    val Navy800 = Color(0xFF0F2262)
    val Navy700 = Color(0xFF143A8F)
    val Blue600 = Color(0xFF1F6FD0)
    val Blue500 = Color(0xFF2F86E0)
    val Blue400 = Color(0xFF5FA8F5)
    val Blue300 = Color(0xFF8EC1F2)
    val Blue200 = Color(0xFFBBD9F3)
    val Blue100 = Color(0xFFDCEBFA)
    val Ice = Color(0xFFF2F6FC)
    val Sky = Color(0xFF86B3EA)
    val Ink = Color(0xFF0B1B4D)
    val Muted = Color(0xFF52669A)
    val Success = Color(0xFF1FB67A)
    val Danger = Color(0xFFE5484D)
    val Warning = Color(0xFFF5A524)
    val Line = Color(0xFFD3E2F5)
}

/** Frosted glass: translucent white gradient (top-left bright, bottom-right clear). */
val GlassFill: Brush = Brush.linearGradient(
    listOf(Color.White.copy(alpha = 0.62f), Color.White.copy(alpha = 0.22f))
)

/** Soft highlight along the top edge of every glass surface. */
val GlassSheen: Brush = Brush.verticalGradient(
    0f to Color.White.copy(alpha = 0.40f),
    0.42f to Color.Transparent,
    1f to Color.Transparent
)

/** Bright rim light that fades out, like the edges of glass. */
val GlassBorder: Brush = Brush.linearGradient(
    listOf(Color.White.copy(alpha = 0.95f), Color.White.copy(alpha = 0.25f))
)

val NavFill: Brush = Brush.verticalGradient(
    listOf(Color.White.copy(alpha = 0.86f), Color.White.copy(alpha = 0.58f))
)

val ButtonBrush: Brush = Brush.linearGradient(listOf(TP.Blue600, TP.Navy900))

val ButtonSheen: Brush = Brush.verticalGradient(
    0f to Color.White.copy(alpha = 0.32f),
    0.55f to Color.Transparent,
    1f to Color.Transparent
)

@Composable
fun TechETTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = lightColorScheme(
            primary = TP.Navy700,
            onPrimary = Color.White,
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

package et.techet.pay.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import et.techet.pay.AppTab
import et.techet.pay.data.moneyParts

/** Full-screen gradient with glowing glass spheres, so translucent cards have something to refract. */
@Composable
fun GlassBackground(content: @Composable BoxScope.() -> Unit) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Brush.verticalGradient(listOf(TP.Ice, TP.Blue200, TP.Sky)))
            .drawBehind {
                val w = size.width
                val h = size.height
                // soft colour washes
                val wash1 = Offset(w * 0.05f, h * 0.04f)
                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(TP.Blue500.copy(alpha = 0.65f), Color.Transparent),
                        center = wash1,
                        radius = w * 0.9f
                    ),
                    radius = w * 0.9f,
                    center = wash1
                )
                val wash2 = Offset(w * 0.1f, h * 0.8f)
                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(TP.Navy700.copy(alpha = 0.45f), Color.Transparent),
                        center = wash2,
                        radius = w * 0.9f
                    ),
                    radius = w * 0.9f,
                    center = wash2
                )
                // glass spheres
                val r1 = w * 0.40f
                val c1 = Offset(w * 1.02f, h * 0.20f)
                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            Color.White.copy(alpha = 0.85f),
                            TP.Blue400.copy(alpha = 0.70f),
                            TP.Blue600.copy(alpha = 0.35f),
                            Color.Transparent
                        ),
                        center = Offset(c1.x - r1 * 0.3f, c1.y - r1 * 0.3f),
                        radius = r1 * 1.2f
                    ),
                    radius = r1,
                    center = c1
                )
                val r2 = w * 0.55f
                val c2 = Offset(w * -0.05f, h * 0.98f)
                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            Color.White.copy(alpha = 0.55f),
                            TP.Blue500.copy(alpha = 0.60f),
                            TP.Navy900.copy(alpha = 0.40f),
                            Color.Transparent
                        ),
                        center = Offset(c2.x - r2 * 0.2f, c2.y - r2 * 0.4f),
                        radius = r2 * 1.2f
                    ),
                    radius = r2,
                    center = c2
                )
            },
        content = content
    )
}

@Composable
fun GlassCard(
    modifier: Modifier = Modifier,
    corner: Dp = 28.dp,
    padding: Dp = 20.dp,
    content: @Composable ColumnScope.() -> Unit
) {
    val shape = RoundedCornerShape(corner)
    Column(
        modifier = modifier
            .fillMaxWidth()
            .clip(shape)
            .background(GlassFill)
            .background(GlassSheen)
            .border(1.dp, GlassBorder, shape)
            .padding(padding),
        content = content
    )
}

/** Dark translucent navy sheet with a grabber, like the Transactions panel in the reference. */
@Composable
fun NavyPanel(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    val shape = RoundedCornerShape(topStart = 34.dp, topEnd = 34.dp, bottomStart = 28.dp, bottomEnd = 28.dp)
    Column(
        modifier = modifier
            .fillMaxWidth()
            .clip(shape)
            .background(
                Brush.verticalGradient(
                    listOf(TP.Navy700.copy(alpha = 0.86f), TP.Navy900.copy(alpha = 0.94f))
                )
            )
            .drawBehind {
                drawRect(
                    brush = Brush.radialGradient(
                        colors = listOf(TP.Blue500.copy(alpha = 0.45f), Color.Transparent),
                        center = Offset(size.width, 0f),
                        radius = size.width * 0.9f
                    )
                )
            }
            .background(
                Brush.verticalGradient(
                    0f to Color.White.copy(alpha = 0.14f),
                    0.3f to Color.Transparent,
                    1f to Color.Transparent
                )
            )
            .border(
                1.dp,
                Brush.linearGradient(listOf(Color.White.copy(alpha = 0.40f), Color.White.copy(alpha = 0.05f))),
                shape
            )
            .padding(start = 18.dp, end = 18.dp, top = 10.dp, bottom = 12.dp)
    ) {
        Box(
            modifier = Modifier
                .align(Alignment.CenterHorizontally)
                .width(44.dp)
                .height(5.dp)
                .clip(RoundedCornerShape(50))
                .background(Color.White.copy(alpha = 0.32f))
        )
        Spacer(Modifier.height(12.dp))
        content()
    }
}

@Composable
fun StatusPill(status: String) {
    val fg: Color
    val bg: Color
    when (status) {
        "success", "delivered" -> { fg = Color(0xFF0B7A53); bg = Color(0xFFD8F5EA) }
        "failed" -> { fg = Color(0xFFB4232A); bg = Color(0xFFFDE2E3) }
        else -> { fg = Color(0xFF8A5A00); bg = Color(0xFFFDEFD0) }
    }
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(50))
            .background(bg.copy(alpha = 0.95f))
            .padding(horizontal = 10.dp, vertical = 3.dp)
    ) {
        Text(
            text = status.replaceFirstChar { it.uppercase() },
            color = fg,
            fontSize = 11.sp,
            fontWeight = FontWeight.Bold
        )
    }
}

@Composable
fun MoneyText(amount: String, bigSp: Int, color: Color, smallColor: Color, currency: String = "ETB") {
    val parts = moneyParts(amount)
    Text(
        text = buildAnnotatedString {
            append(parts.first)
            withStyle(SpanStyle(fontSize = (bigSp * 0.6f).sp, color = smallColor, fontWeight = FontWeight.Bold)) {
                append("." + parts.second)
            }
            if (currency.isNotEmpty()) {
                withStyle(SpanStyle(fontSize = (bigSp * 0.38f).sp, color = smallColor, fontWeight = FontWeight.SemiBold)) {
                    append("  " + currency)
                }
            }
        },
        color = color,
        fontSize = bigSp.sp,
        fontWeight = FontWeight.ExtraBold
    )
}

@Composable
fun GlassChip(text: String) {
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(50))
            .background(
                Brush.verticalGradient(listOf(Color.White.copy(alpha = 0.26f), Color.White.copy(alpha = 0.10f)))
            )
            .border(1.dp, Color.White.copy(alpha = 0.28f), RoundedCornerShape(50))
            .padding(horizontal = 12.dp, vertical = 6.dp)
    ) {
        Text(text, color = Color.White, fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold)
    }
}

@Composable
fun HeroBalanceCard(balance: String, chips: List<String>) {
    val shape = RoundedCornerShape(28.dp)
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .shadow(16.dp, shape, ambientColor = TP.Navy900, spotColor = TP.Navy700)
            .clip(shape)
            .background(Brush.linearGradient(listOf(TP.Navy700, TP.Navy900)))
            .drawBehind {
                drawRect(
                    brush = Brush.radialGradient(
                        colors = listOf(TP.Blue400.copy(alpha = 0.80f), Color.Transparent),
                        center = Offset(size.width, 0f),
                        radius = size.width * 0.95f
                    )
                )
                drawRect(
                    brush = Brush.radialGradient(
                        colors = listOf(TP.Blue500.copy(alpha = 0.35f), Color.Transparent),
                        center = Offset(0f, size.height),
                        radius = size.width * 0.7f
                    )
                )
            }
            .background(
                Brush.linearGradient(
                    0f to Color.White.copy(alpha = 0.22f),
                    0.4f to Color.Transparent,
                    1f to Color.Transparent
                )
            )
            .border(
                1.dp,
                Brush.linearGradient(listOf(Color.White.copy(alpha = 0.55f), Color.White.copy(alpha = 0.06f))),
                shape
            )
            .padding(22.dp)
    ) {
        Text("Available balance", color = TP.Blue200, fontSize = 13.sp)
        Spacer(Modifier.height(4.dp))
        MoneyText(balance, 38, Color.White, TP.Blue300)
        Spacer(Modifier.height(12.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            for (c in chips) GlassChip(c)
        }
    }
}

@Composable
fun ActivityTile(label: String, icon: ImageVector, onClick: () -> Unit, modifier: Modifier = Modifier) {
    val shape = RoundedCornerShape(22.dp)
    Column(
        modifier = modifier.clickable(onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Box(
            modifier = Modifier
                .size(68.dp)
                .shadow(10.dp, shape, ambientColor = TP.Navy900, spotColor = TP.Navy900)
                .clip(shape)
                .background(Brush.linearGradient(listOf(TP.Blue500, TP.Navy900)))
                .background(
                    Brush.verticalGradient(
                        0f to Color.White.copy(alpha = 0.34f),
                        0.55f to Color.Transparent,
                        1f to Color.Transparent
                    )
                )
                .border(1.dp, Color.White.copy(alpha = 0.40f), shape),
            contentAlignment = Alignment.Center
        ) {
            Icon(icon, contentDescription = label, tint = Color.White, modifier = Modifier.size(28.dp))
        }
        Spacer(Modifier.height(8.dp))
        Text(label, color = TP.Ink, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
    }
}

@Composable
fun FloatingNavBar(
    selected: AppTab,
    onSelect: (AppTab) -> Unit,
    onPlus: () -> Unit,
    modifier: Modifier = Modifier
) {
    val pill = RoundedCornerShape(33.dp)
    Box(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp)
            .navigationBarsPadding()
            .padding(bottom = 10.dp)
            .height(96.dp)
    ) {
        Row(
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth()
                .height(66.dp)
                .shadow(14.dp, pill, ambientColor = TP.Navy900.copy(alpha = 0.4f), spotColor = TP.Navy900.copy(alpha = 0.4f))
                .clip(pill)
                .background(NavFill)
                .border(1.dp, GlassBorder, pill),
            verticalAlignment = Alignment.CenterVertically
        ) {
            NavItem(Icons.Filled.Home, "Home", selected == AppTab.Home, { onSelect(AppTab.Home) }, Modifier.weight(1f))
            NavItem(Icons.AutoMirrored.Filled.List, "Payments", selected == AppTab.Payments, { onSelect(AppTab.Payments) }, Modifier.weight(1f))
            Spacer(Modifier.width(72.dp))
            NavItem(Icons.AutoMirrored.Filled.Send, "Payouts", selected == AppTab.Payouts, { onSelect(AppTab.Payouts) }, Modifier.weight(1f))
            NavItem(Icons.Filled.Person, "Account", selected == AppTab.Account, { onSelect(AppTab.Account) }, Modifier.weight(1f))
        }
        Box(
            modifier = Modifier
                .align(Alignment.TopCenter)
                .size(66.dp)
                .shadow(14.dp, CircleShape, ambientColor = TP.Navy900, spotColor = TP.Navy900)
                .clip(CircleShape)
                .background(Brush.linearGradient(listOf(TP.Blue500, TP.Navy800)))
                .background(
                    Brush.verticalGradient(
                        0f to Color.White.copy(alpha = 0.40f),
                        0.55f to Color.Transparent,
                        1f to Color.Transparent
                    )
                )
                .border(3.dp, Color.White.copy(alpha = 0.85f), CircleShape)
                .clickable(onClick = onPlus),
            contentAlignment = Alignment.Center
        ) {
            Icon(Icons.Filled.Add, contentDescription = "Create payment link", tint = Color.White, modifier = Modifier.size(30.dp))
        }
    }
}

@Composable
private fun NavItem(icon: ImageVector, label: String, selected: Boolean, onClick: () -> Unit, modifier: Modifier) {
    val tint = if (selected) TP.Navy700 else TP.Muted
    Column(
        modifier = modifier
            .clickable(onClick = onClick)
            .padding(vertical = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Icon(icon, contentDescription = label, tint = tint, modifier = Modifier.size(23.dp))
        Spacer(Modifier.height(2.dp))
        Text(label, color = tint, fontSize = 11.sp, fontWeight = FontWeight.Bold)
    }
}

@Composable
fun LineChart(values: List<Double>, modifier: Modifier = Modifier) {
    Canvas(modifier = modifier.fillMaxWidth().height(150.dp)) {
        if (values.size < 2) return@Canvas
        val w = size.width
        val h = size.height
        val padX = 12.dp.toPx()
        val top = 14.dp.toPx()
        val bottom = 8.dp.toPx()
        val max = (values.maxOrNull() ?: 0.0).coerceAtLeast(1.0)
        val n = values.size
        val xs = FloatArray(n) { i -> padX + i * (w - 2 * padX) / (n - 1) }
        val ys = FloatArray(n) { i -> h - bottom - (values[i] / max).toFloat() * (h - top - bottom) }
        val line = Path()
        line.moveTo(xs[0], ys[0])
        for (i in 0 until n - 1) {
            val x0 = xs[maxOf(i - 1, 0)]
            val y0 = ys[maxOf(i - 1, 0)]
            val x1 = xs[i]
            val y1 = ys[i]
            val x2 = xs[i + 1]
            val y2 = ys[i + 1]
            val x3 = xs[minOf(i + 2, n - 1)]
            val y3 = ys[minOf(i + 2, n - 1)]
            val c1x = x1 + (x2 - x0) / 6f
            val c1y = (y1 + (y2 - y0) / 6f).coerceIn(top, h - bottom)
            val c2x = x2 - (x3 - x1) / 6f
            val c2y = (y2 - (y3 - y1) / 6f).coerceIn(top, h - bottom)
            line.cubicTo(c1x, c1y, c2x, c2y, x2, y2)
        }
        val fill = Path()
        fill.addPath(line)
        fill.lineTo(xs[n - 1], h - bottom)
        fill.lineTo(xs[0], h - bottom)
        fill.close()
        drawPath(
            path = fill,
            brush = Brush.verticalGradient(listOf(TP.Blue500.copy(alpha = 0.38f), Color.Transparent))
        )
        drawPath(
            path = line,
            color = TP.Blue600,
            style = Stroke(width = 3.5.dp.toPx(), cap = StrokeCap.Round)
        )
        val last = Offset(xs[n - 1], ys[n - 1])
        drawCircle(color = Color.White, radius = 6.dp.toPx(), center = last)
        drawCircle(color = TP.Navy700, radius = 6.dp.toPx(), center = last, style = Stroke(width = 3.dp.toPx()))
    }
}

@Composable
fun TechField(
    value: String,
    onChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    keyboard: KeyboardType = KeyboardType.Text,
    password: Boolean = false,
    placeholder: String = ""
) {
    val shape = RoundedCornerShape(16.dp)
    Column(modifier = modifier.fillMaxWidth()) {
        Text(
            text = label,
            color = TP.Navy800,
            fontSize = 13.sp,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.padding(top = 14.dp, bottom = 6.dp)
        )
        BasicTextField(
            value = value,
            onValueChange = onChange,
            singleLine = true,
            textStyle = TextStyle(color = TP.Ink, fontSize = 16.sp),
            keyboardOptions = KeyboardOptions(keyboardType = keyboard),
            visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
            cursorBrush = SolidColor(TP.Blue600),
            modifier = Modifier.fillMaxWidth(),
            decorationBox = { inner ->
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clip(shape)
                        .background(Color.White.copy(alpha = 0.62f))
                        .border(1.5.dp, Color.White.copy(alpha = 0.9f), shape)
                        .padding(horizontal = 16.dp, vertical = 14.dp)
                ) {
                    if (value.isEmpty() && placeholder.isNotEmpty()) {
                        Text(placeholder, color = TP.Muted.copy(alpha = 0.7f), fontSize = 16.sp)
                    }
                    inner()
                }
            }
        )
    }
}

@Composable
fun PrimaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false
) {
    val shape = RoundedCornerShape(50)
    Box(
        modifier = modifier
            .height(52.dp)
            .alpha(if (enabled && !loading) 1f else 0.6f)
            .clip(shape)
            .background(ButtonBrush)
            .background(ButtonSheen)
            .border(1.dp, Color.White.copy(alpha = 0.35f), shape)
            .clickable(enabled = enabled && !loading, onClick = onClick)
            .padding(horizontal = 24.dp),
        contentAlignment = Alignment.Center
    ) {
        if (loading) {
            CircularProgressIndicator(
                modifier = Modifier.size(22.dp),
                color = Color.White,
                strokeWidth = 2.dp
            )
        } else {
            Text(text, color = Color.White, fontWeight = FontWeight.Bold, fontSize = 15.sp)
        }
    }
}

@Composable
fun GhostButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier) {
    val shape = RoundedCornerShape(50)
    Box(
        modifier = modifier
            .height(48.dp)
            .clip(shape)
            .background(GlassFill)
            .border(1.dp, GlassBorder, shape)
            .clickable(onClick = onClick)
            .padding(horizontal = 22.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(text, color = TP.Navy800, fontWeight = FontWeight.Bold, fontSize = 14.sp)
    }
}

@Composable
fun PickChip(text: String, selected: Boolean, onClick: () -> Unit) {
    val shape = RoundedCornerShape(50)
    val base = if (selected) Modifier
        .background(ButtonBrush)
        .background(ButtonSheen)
    else Modifier.background(GlassFill)
    Box(
        modifier = Modifier
            .clip(shape)
            .then(base)
            .border(1.dp, if (selected) Color.White.copy(alpha = 0.35f) else Color.White.copy(alpha = 0.9f), shape)
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 10.dp)
    ) {
        Text(
            text,
            color = if (selected) Color.White else TP.Navy800,
            fontSize = 14.sp,
            fontWeight = FontWeight.SemiBold
        )
    }
}

@Composable
fun LogoMark(size: Dp) {
    val shape = RoundedCornerShape(size * 0.3f)
    Box(
        modifier = Modifier
            .size(size)
            .clip(shape)
            .background(Brush.linearGradient(listOf(TP.Blue600, TP.Navy900)))
            .background(ButtonSheen)
            .border(1.dp, Color.White.copy(alpha = 0.4f), shape),
        contentAlignment = Alignment.Center
    ) {
        Text("T", color = Color.White, fontWeight = FontWeight.ExtraBold, fontSize = (size.value * 0.5f).sp)
    }
}

@Composable
fun GlassDialog(onDismiss: () -> Unit, content: @Composable ColumnScope.() -> Unit) {
    Dialog(onDismissRequest = onDismiss) {
        val shape = RoundedCornerShape(30.dp)
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .clip(shape)
                .background(Brush.linearGradient(listOf(Color.White.copy(alpha = 0.96f), TP.Blue100.copy(alpha = 0.92f))))
                .background(GlassSheen)
                .border(1.dp, GlassBorder, shape)
                .padding(22.dp),
            content = content
        )
    }
}

@Composable
fun Divider(modifier: Modifier = Modifier, color: Color = Color.White.copy(alpha = 0.12f)) {
    Box(modifier = modifier.fillMaxWidth().height(1.dp).background(color))
}

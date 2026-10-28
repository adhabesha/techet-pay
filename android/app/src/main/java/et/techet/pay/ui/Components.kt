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
import androidx.compose.ui.res.painterResource
import et.techet.pay.R
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

/** Full-screen background: flat light (default) or near-black surface. No gradients. */
@Composable
fun GlassBackground(content: @Composable BoxScope.() -> Unit) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(TP.Ice),
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

/** Flat black panel with a grabber, like the transactions card on the landing page. */
@Composable
fun NavyPanel(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    val shape = RoundedCornerShape(28.dp)
    Column(
        modifier = modifier
            .fillMaxWidth()
            .clip(shape)
            .background(TP.Navy900)
            .padding(start = 18.dp, end = 18.dp, top = 10.dp, bottom = 12.dp)
    ) {
        Box(
            modifier = Modifier
                .align(Alignment.CenterHorizontally)
                .width(44.dp)
                .height(5.dp)
                .clip(RoundedCornerShape(50))
                .background(Color.White.copy(alpha = 0.22f))
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
            .background(TP.Navy900)
            .padding(horizontal = 12.dp, vertical = 6.dp)
    ) {
        Text(text, color = Color.White, fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold)
    }
}

@Composable
fun HeroBalanceCard(balance: String, chips: List<String>) {
    val shape = RoundedCornerShape(30.dp)
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(shape)
            .background(TP.Blue400)
            .drawBehind {
                // flat concentric rings, no gradient
                val c = Offset(size.width * 0.86f, size.height * 0.12f)
                for (i in 1..5) {
                    drawCircle(
                        color = Color(0xFF111312).copy(alpha = 0.07f),
                        radius = size.width * 0.12f * i,
                        center = c,
                        style = Stroke(width = 1.5.dp.toPx())
                    )
                }
            }
            .padding(22.dp)
    ) {
        Text("Available balance", color = TP.Navy900.copy(alpha = 0.7f), fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(4.dp))
        MoneyText(balance, 38, TP.Navy900, TP.Navy900.copy(alpha = 0.7f))
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
                .clip(shape)
                .background(TP.Navy900),
            contentAlignment = Alignment.Center
        ) {
            Icon(icon, contentDescription = label, tint = TP.Blue400, modifier = Modifier.size(28.dp))
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
                .clip(pill)
                .background(TP.Navy900),
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
                .clip(CircleShape)
                .background(TP.Ice)
                .padding(5.dp)
                .clip(CircleShape)
                .background(TP.Blue400)
                .clickable(onClick = onPlus),
            contentAlignment = Alignment.Center
        ) {
            Icon(Icons.Filled.Add, contentDescription = "Create payment link", tint = TP.Navy800, modifier = Modifier.size(30.dp))
        }
    }
}

@Composable
private fun NavItem(icon: ImageVector, label: String, selected: Boolean, onClick: () -> Unit, modifier: Modifier) {
    Box(modifier = modifier.clickable(onClick = onClick), contentAlignment = Alignment.Center) {
        if (selected) {
            Row(
                modifier = Modifier
                    .clip(RoundedCornerShape(50))
                    .background(TP.Blue400)
                    .padding(horizontal = 12.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Icon(icon, contentDescription = label, tint = TP.Navy900, modifier = Modifier.size(22.dp))
                Spacer(Modifier.width(6.dp))
                Text(label, color = TP.Navy900, fontSize = 12.sp, fontWeight = FontWeight.ExtraBold, maxLines = 1)
            }
        } else {
            Icon(icon, contentDescription = label, tint = Color.White.copy(alpha = 0.65f), modifier = Modifier.size(23.dp))
        }
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
            color = TP.Blue400.copy(alpha = 0.18f)
        )
        drawPath(
            path = line,
            color = if (ThemeState.dark) TP.Blue400 else TP.Navy900,
            style = Stroke(width = 3.5.dp.toPx(), cap = StrokeCap.Round)
        )
        val last = Offset(xs[n - 1], ys[n - 1])
        drawCircle(color = TP.Blue400, radius = 6.dp.toPx(), center = last)
        drawCircle(color = TP.Navy900, radius = 6.dp.toPx(), center = last, style = Stroke(width = 3.dp.toPx()))
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
            color = TP.Ink,
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
                        .background(TP.Card)
                        .border(1.5.dp, TP.Line, shape)
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
                color = TP.Navy800,
                strokeWidth = 2.dp
            )
        } else {
            Text(text, color = TP.Navy800, fontWeight = FontWeight.Bold, fontSize = 15.sp)
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
        Text(text, color = TP.Ink, fontWeight = FontWeight.Bold, fontSize = 14.sp)
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
            .border(1.dp, if (selected) Color.Transparent else TP.Line, shape)
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 10.dp)
    ) {
        Text(
            text,
            color = if (selected) TP.Navy800 else TP.Ink,
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
            .background(TP.Blue400)
            .background(ButtonSheen)
            .border(1.dp, Color.White.copy(alpha = 0.4f), shape),
        contentAlignment = Alignment.Center
    ) {
        Icon(painterResource(R.drawable.ic_mark), contentDescription = null, tint = Color.White, modifier = Modifier.size(size * 0.56f))
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
                .background(Brush.linearGradient(listOf(TP.Card, TP.Card)))
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

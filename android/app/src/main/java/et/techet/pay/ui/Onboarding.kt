package et.techet.pay.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Share
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

private class Slide(val icon: ImageVector, val title: String, val body: String)

private val slides = listOf(
    Slide(Icons.Filled.Share, "Get paid anywhere in Ethiopia", "Create a payment link in seconds and share it with any customer on any phone."),
    Slide(Icons.AutoMirrored.Filled.List, "Every wallet, one checkout", "Telebirr, CBE Birr, M-Pesa, Amole and cards, all in a single hosted page."),
    Slide(Icons.Filled.Refresh, "Watch every birr live", "See balance, fees and payments update as they happen, with webhooks to your app."),
    Slide(Icons.AutoMirrored.Filled.Send, "Withdraw to your bank", "Request a payout to any Ethiopian bank whenever you like. Simple, tracked, fast.")
)

/** First-launch intro. [onDone] is called when the user finishes or skips. */
@Composable
fun OnboardingScreen(onDone: () -> Unit) {
    var page by remember { mutableIntStateOf(0) }
    val s = slides[page]
    val last = page == slides.size - 1
    Column(
        modifier = Modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding()
            .padding(horizontal = 22.dp, vertical = 14.dp)
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.fillMaxWidth()) {
            LogoMark(40.dp)
            Spacer(Modifier.width(10.dp))
            Text("Xidex", color = TP.Ink, fontSize = 22.sp, fontWeight = FontWeight.ExtraBold, modifier = Modifier.weight(1f))
            if (!last) {
                Text(
                    "Skip",
                    color = TP.Ink,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier
                        .clip(RoundedCornerShape(50))
                        .clickable(onClick = onDone)
                        .padding(horizontal = 14.dp, vertical = 8.dp)
                )
            }
        }
        Spacer(Modifier.height(18.dp))
        Box(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .clip(RoundedCornerShape(34.dp))
                .background(TP.Blue400),
            contentAlignment = Alignment.Center
        ) {
            Box(
                modifier = Modifier
                    .size(132.dp)
                    .clip(RoundedCornerShape(40.dp))
                    .background(TP.Navy900),
                contentAlignment = Alignment.Center
            ) {
                Icon(s.icon, contentDescription = null, tint = TP.Blue400, modifier = Modifier.size(60.dp))
            }
        }
        Spacer(Modifier.height(22.dp))
        Text(s.title, color = TP.Ink, fontSize = 30.sp, lineHeight = 34.sp, fontWeight = FontWeight.ExtraBold)
        Spacer(Modifier.height(8.dp))
        Text(s.body, color = TP.Muted, fontSize = 16.sp, lineHeight = 22.sp)
        Spacer(Modifier.height(18.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            for (i in slides.indices) {
                Box(
                    modifier = Modifier
                        .height(8.dp)
                        .width(if (i == page) 28.dp else 8.dp)
                        .clip(CircleShape)
                        .background(if (i == page) TP.Blue500 else TP.Line)
                )
            }
        }
        Spacer(Modifier.height(18.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.fillMaxWidth()) {
            if (page > 0) GhostButton("Back", { page -= 1 })
            PrimaryButton(
                if (last) "Get started" else "Next",
                { if (last) onDone() else page += 1 },
                Modifier.weight(1f)
            )
        }
    }
}

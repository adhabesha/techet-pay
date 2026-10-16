package et.techet.pay.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Share
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import et.techet.pay.AppTab
import et.techet.pay.AppViewModel
import et.techet.pay.data.Tx
import et.techet.pay.data.chartValue
import et.techet.pay.data.formatDate
import et.techet.pay.data.formatMoney

@Composable
fun Root(vm: AppViewModel) {
    if (vm.booting) {
        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                LogoMark(72.dp)
                Spacer(Modifier.height(16.dp))
                Text("TechET Pay", color = TP.Navy800, fontSize = 24.sp, fontWeight = FontWeight.ExtraBold)
                Spacer(Modifier.height(6.dp))
                Text("Connecting to the server…", color = TP.Muted, fontSize = 14.sp)
            }
        }
    } else if (vm.merchant == null) {
        AuthScreen(vm)
    } else {
        MainShell(vm)
    }
}

// ---------------------------------------------------------------- auth

@Composable
fun AuthScreen(vm: AppViewModel) {
    var registerMode by remember { mutableStateOf(false) }
    var name by remember { mutableStateOf("") }
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var showServer by remember { mutableStateOf(false) }
    var server by remember { mutableStateOf(vm.serverUrl) }
    var serverError by remember { mutableStateOf<String?>(null) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding()
            .imePadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp, vertical = 24.dp)
    ) {
        Spacer(Modifier.height(24.dp))
        Row(verticalAlignment = Alignment.CenterVertically) {
            LogoMark(48.dp)
            Spacer(Modifier.width(12.dp))
            Text("TechET Pay", color = TP.Navy800, fontSize = 28.sp, fontWeight = FontWeight.ExtraBold)
            Spacer(Modifier.weight(1f))
            Box(
                modifier = Modifier
                    .clip(RoundedCornerShape(50))
                    .background(Color(0xFFFDEFD0))
                    .padding(horizontal = 10.dp, vertical = 4.dp)
            ) {
                Text("SANDBOX", color = Color(0xFF8A5A00), fontSize = 11.sp, fontWeight = FontWeight.ExtraBold)
            }
        }
        Text(
            "Accept Telebirr, CBE Birr, M-Pesa, Amole and cards.",
            color = TP.Muted,
            modifier = Modifier.padding(top = 8.dp, bottom = 20.dp)
        )
        GlassCard {
            val seg = RoundedCornerShape(50)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(seg)
                    .background(GlassFill)
                    .border(1.dp, GlassBorder, seg)
                    .padding(5.dp)
            ) {
                SegButton("Sign in", !registerMode, { registerMode = false }, Modifier.weight(1f))
                SegButton("Create account", registerMode, { registerMode = true }, Modifier.weight(1f))
            }
            if (registerMode) {
                TechField(name, { name = it }, "Business name")
            }
            TechField(email, { email = it }, "Email", keyboard = KeyboardType.Email)
            TechField(password, { password = it }, "Password", keyboard = KeyboardType.Password, password = true)
            val err = vm.authError
            if (err != null) {
                Text(err, color = TP.Danger, fontSize = 14.sp, modifier = Modifier.padding(top = 10.dp))
            }
            Spacer(Modifier.height(18.dp))
            PrimaryButton(
                text = if (registerMode) "Create merchant account" else "Sign in",
                onClick = {
                    if (registerMode) vm.register(name, email, password) else vm.login(email, password)
                },
                modifier = Modifier.fillMaxWidth(),
                loading = vm.busy > 0
            )
            if (vm.busy > 0) {
                Text(
                    "Free servers sleep when idle. The first request can take up to a minute.",
                    color = TP.Muted,
                    fontSize = 12.sp,
                    modifier = Modifier.padding(top = 10.dp)
                )
            }
        }
        Spacer(Modifier.height(16.dp))
        Text(
            text = if (showServer) "Hide server settings" else "Server settings",
            color = TP.Navy700,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier
                .clickable { showServer = !showServer }
                .padding(8.dp)
        )
        if (showServer) {
            GlassCard {
                TechField(server, { server = it }, "Server address", keyboard = KeyboardType.Uri, placeholder = "https://your-server.onrender.com")
                val se = serverError
                if (se != null) Text(se, color = TP.Danger, fontSize = 13.sp, modifier = Modifier.padding(top = 8.dp))
                Spacer(Modifier.height(12.dp))
                GhostButton("Save server", {
                    serverError = vm.saveServerUrl(server)
                    if (serverError == null) server = vm.serverUrl
                })
            }
        }
    }
}

@Composable
private fun SegButton(text: String, selected: Boolean, onClick: () -> Unit, modifier: Modifier) {
    val shape = RoundedCornerShape(50)
    val base = if (selected) Modifier
        .background(ButtonBrush)
        .background(ButtonSheen)
    else Modifier
    Box(
        modifier = modifier
            .clip(shape)
            .then(base)
            .clickable(onClick = onClick)
            .padding(vertical = 11.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text,
            color = if (selected) Color.White else TP.Muted,
            fontWeight = FontWeight.Bold,
            fontSize = 14.sp
        )
    }
}

// ---------------------------------------------------------------- shell

@Composable
fun MainShell(vm: AppViewModel) {
    var showCreate by remember { mutableStateOf(false) }
    var selectedTx by remember { mutableStateOf<Tx?>(null) }
    val merchant = vm.merchant

    BackHandler(enabled = vm.tab != AppTab.Home) { vm.tab = AppTab.Home }

    Box(modifier = Modifier.fillMaxSize()) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .statusBarsPadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp)
        ) {
            Row(
                modifier = Modifier.padding(top = 8.dp, bottom = 14.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                LogoMark(44.dp)
                Spacer(Modifier.width(12.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        when (vm.tab) {
                            AppTab.Home -> "Welcome back"
                            AppTab.Payments -> "Checkout transactions"
                            AppTab.Payouts -> "Withdraw to a bank or wallet"
                            AppTab.Account -> "Keys, webhooks and settings"
                        },
                        color = TP.Muted,
                        fontSize = 13.sp
                    )
                    Text(
                        when (vm.tab) {
                            AppTab.Home -> merchant?.name ?: ""
                            AppTab.Payments -> "Payments"
                            AppTab.Payouts -> "Payouts"
                            AppTab.Account -> "Account"
                        },
                        color = TP.Ink,
                        fontSize = 20.sp,
                        fontWeight = FontWeight.Bold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
                val rs = RoundedCornerShape(16.dp)
                Box(
                    modifier = Modifier
                        .size(44.dp)
                        .clip(rs)
                        .background(GlassFill)
                        .border(1.dp, GlassBorder, rs)
                        .clickable { vm.refreshAll() },
                    contentAlignment = Alignment.Center
                ) {
                    Icon(Icons.Filled.Refresh, contentDescription = "Refresh", tint = TP.Navy700, modifier = Modifier.size(22.dp))
                }
            }
            if (vm.busy > 0) {
                LinearProgressIndicator(
                    modifier = Modifier.fillMaxWidth().padding(bottom = 10.dp),
                    color = TP.Blue600,
                    trackColor = Color.White.copy(alpha = 0.5f)
                )
            }
            val err = vm.error
            if (err != null) {
                Text(err, color = TP.Danger, fontSize = 14.sp, modifier = Modifier.padding(bottom = 10.dp))
            }

            when (vm.tab) {
                AppTab.Home -> HomeContent(vm, onCreate = { showCreate = true }, onOpenTx = { selectedTx = it })
                AppTab.Payments -> PaymentsContent(vm, onOpenTx = { selectedTx = it })
                AppTab.Payouts -> PayoutsContent(vm)
                AppTab.Account -> AccountContent(vm)
            }
            Spacer(Modifier.height(130.dp))
        }

        FloatingNavBar(
            selected = vm.tab,
            onSelect = { vm.tab = it },
            onPlus = { showCreate = true },
            modifier = Modifier.align(Alignment.BottomCenter)
        )
    }

    if (showCreate) {
        CreatePaymentDialog(vm, onDismiss = { showCreate = false })
    }
    val tx = selectedTx
    if (tx != null) {
        TxDetailDialog(tx, onDismiss = { selectedTx = null })
    }
    val key = vm.revealedKey
    if (key != null) {
        SecretKeyDialog(key, onDone = { vm.revealedKey = null })
    }
}

// ---------------------------------------------------------------- home

@Composable
fun HomeContent(vm: AppViewModel, onCreate: () -> Unit, onOpenTx: (Tx) -> Unit) {
    val o = vm.overview
    HeroBalanceCard(
        balance = o?.availableBalance ?: "0.00",
        chips = listOf(
            "Net " + formatMoney(o?.netEarned ?: "0.00"),
            "Fee " + (o?.feePercent ?: 0.0) + "%",
            (o?.successful ?: 0).toString() + " paid"
        )
    )
    Spacer(Modifier.height(18.dp))
    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly) {
        ActivityTile("Pay link", Icons.Filled.Share, onCreate)
        ActivityTile("Payout", Icons.AutoMirrored.Filled.Send, { vm.tab = AppTab.Payouts })
        ActivityTile("History", Icons.AutoMirrored.Filled.List, { vm.tab = AppTab.Payments })
        ActivityTile("Keys", Icons.Filled.Lock, { vm.tab = AppTab.Account })
    }
    Spacer(Modifier.height(18.dp))
    GlassCard {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("Statistic", color = TP.Ink, fontSize = 17.sp, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
            Text("Last 14 days", color = TP.Navy700, fontSize = 12.sp, fontWeight = FontWeight.SemiBold)
        }
        Spacer(Modifier.height(8.dp))
        val series = o?.series ?: emptyList()
        LineChart(values = series.map { chartValue(it.volume) })
        if (series.size >= 2) {
            Row(modifier = Modifier.fillMaxWidth().padding(top = 4.dp), horizontalArrangement = Arrangement.SpaceBetween) {
                Text(series.first().date.takeLast(5), color = TP.Muted, fontSize = 11.sp)
                Text(series[series.size / 2].date.takeLast(5), color = TP.Muted, fontSize = 11.sp)
                Text(series.last().date.takeLast(5), color = TP.Muted, fontSize = 11.sp)
            }
        }
        Spacer(Modifier.height(14.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            StatTile("Income", formatMoney(o?.grossVolume ?: "0.00") + " ETB", true, Modifier.weight(1f))
            StatTile("Pending / failed", (o?.pending ?: 0).toString() + " / " + (o?.failed ?: 0).toString(), false, Modifier.weight(1f))
        }
    }
    Spacer(Modifier.height(18.dp))
    NavyPanel {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("Transactions", color = Color.White, fontSize = 17.sp, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
            Text(
                "See all",
                color = TP.Blue200,
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.clickable { vm.tab = AppTab.Payments }.padding(6.dp)
            )
        }
        TxList(vm.txs.take(5), onOpenTx)
    }
}

@Composable
private fun StatTile(label: String, value: String, dark: Boolean, modifier: Modifier) {
    val shape = RoundedCornerShape(22.dp)
    val brush = if (dark) Brush.linearGradient(listOf(TP.Navy700, TP.Blue600))
    else Brush.linearGradient(listOf(TP.Blue400.copy(alpha = 0.92f), TP.Blue200.copy(alpha = 0.88f)))
    val fg = if (dark) Color.White else TP.Navy900
    Column(
        modifier = modifier
            .clip(shape)
            .background(brush)
            .background(
                Brush.linearGradient(
                    0f to Color.White.copy(alpha = if (dark) 0.26f else 0.45f),
                    0.45f to Color.Transparent,
                    1f to Color.Transparent
                )
            )
            .border(1.dp, Color.White.copy(alpha = 0.35f), shape)
            .padding(16.dp)
    ) {
        Text(label, color = fg.copy(alpha = 0.85f), fontSize = 12.5.sp, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(2.dp))
        Text(value, color = fg, fontSize = 17.sp, fontWeight = FontWeight.ExtraBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
fun TxList(list: List<Tx>, onOpenTx: (Tx) -> Unit) {
    if (list.isEmpty()) {
        Text(
            "No payments yet. Tap + to create a test payment.",
            color = TP.Blue300,
            modifier = Modifier.fillMaxWidth().padding(vertical = 26.dp)
        )
        return
    }
    for ((i, t) in list.withIndex()) {
        if (i > 0) Divider()
        TxRow(t, onClick = { onOpenTx(t) })
    }
}

@Composable
fun TxRow(t: Tx, onClick: () -> Unit) {
    val shape = RoundedCornerShape(15.dp)
    Row(
        modifier = Modifier.fillMaxWidth().clickable(onClick = onClick).padding(vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            modifier = Modifier
                .size(42.dp)
                .clip(shape)
                .background(Brush.verticalGradient(listOf(Color.White.copy(alpha = 0.26f), Color.White.copy(alpha = 0.08f))))
                .border(1.dp, Color.White.copy(alpha = 0.22f), shape),
            contentAlignment = Alignment.Center
        ) {
            Text(
                if (t.method.isEmpty()) "--" else t.method.take(2).uppercase(),
                color = TP.Blue200,
                fontWeight = FontWeight.ExtraBold,
                fontSize = 13.sp
            )
        }
        Spacer(Modifier.width(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(
                if (t.email.isNotEmpty()) t.email else t.txRef,
                color = Color.White,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                formatDate(t.createdAt) + " · " + (if (t.method.isEmpty()) "not started" else t.method),
                color = TP.Blue300,
                fontSize = 12.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
        Spacer(Modifier.width(8.dp))
        Column(horizontalAlignment = Alignment.End) {
            Text(formatMoney(t.amount), color = Color.White, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(3.dp))
            StatusPill(t.status)
        }
    }
}

// ---------------------------------------------------------------- payments

@Composable
fun PaymentsContent(vm: AppViewModel, onOpenTx: (Tx) -> Unit) {
    var filter by remember { mutableStateOf("all") }
    Row(
        modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        for (f in listOf("all", "success", "pending", "failed")) {
            PickChip(f.replaceFirstChar { it.uppercase() }, filter == f, { filter = f })
        }
    }
    Spacer(Modifier.height(14.dp))
    val list = if (filter == "all") vm.txs else vm.txs.filter { it.status == filter }
    NavyPanel {
        TxList(list, onOpenTx)
    }
}

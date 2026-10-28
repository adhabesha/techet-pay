package et.techet.pay.ui

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
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
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import et.techet.pay.AppViewModel
import et.techet.pay.data.Transfer
import et.techet.pay.data.Tx
import et.techet.pay.data.formatDate
import et.techet.pay.data.formatMoney

// ---------------------------------------------------------------- payouts

@Composable
fun PayoutsContent(vm: AppViewModel) {
    var bankCode by remember { mutableStateOf("") }
    var amount by remember { mutableStateOf("") }
    var holder by remember { mutableStateOf("") }
    var account by remember { mutableStateOf("") }
    val o = vm.overview
    val selected = if (bankCode.isEmpty() && vm.banks.isNotEmpty()) vm.banks[0].code else bankCode

    HeroBalanceCard(
        balance = o?.availableBalance ?: "0.00",
        chips = listOf("Available to withdraw", "Fee " + formatMoney(o?.payoutFee ?: "0.00") + " ETB")
    )
    Spacer(Modifier.height(18.dp))
    GlassCard {
        Text("New payout", color = TP.Ink, fontSize = 17.sp, fontWeight = FontWeight.Bold)
        Text(
            "Send to",
            color = TP.Ink,
            fontSize = 13.sp,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.padding(top = 14.dp, bottom = 8.dp)
        )
        Row(
            modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            for (b in vm.banks) {
                PickChip(b.name, selected == b.code, { bankCode = b.code })
            }
        }
        TechField(amount, { amount = it }, "Amount (ETB)", keyboard = KeyboardType.Decimal, placeholder = "500.00")
        TechField(holder, { holder = it }, "Account holder name")
        TechField(account, { account = it }, "Account or phone number", keyboard = KeyboardType.Number)
        val pe = vm.payoutError
        if (pe != null) Text(pe, color = TP.Danger, fontSize = 14.sp, modifier = Modifier.padding(top = 10.dp))
        val pn = vm.payoutNotice
        if (pn != null) Text(pn, color = Color(0xFF0B7A53), fontSize = 14.sp, modifier = Modifier.padding(top = 10.dp))
        Spacer(Modifier.height(16.dp))
        PrimaryButton(
            text = "Request payout (fee " + formatMoney(o?.payoutFee ?: "0.00") + " ETB)",
            onClick = { vm.requestPayout(selected, amount, holder, account) },
            modifier = Modifier.fillMaxWidth(),
            enabled = selected.isNotEmpty(),
            loading = vm.busy > 0
        )
        Text(
            "Sandbox: account numbers ending in 0000 are rejected and refunded automatically.",
            color = TP.Muted,
            fontSize = 12.sp,
            modifier = Modifier.padding(top = 10.dp)
        )
    }
    Spacer(Modifier.height(18.dp))
    NavyPanel {
        Text("History", color = Color.White, fontSize = 17.sp, fontWeight = FontWeight.Bold)
        if (vm.transfers.isEmpty()) {
            Text("No payouts yet.", color = TP.Blue300, modifier = Modifier.fillMaxWidth().padding(vertical = 26.dp))
        }
        for ((i, t) in vm.transfers.withIndex()) {
            if (i > 0) Divider()
            TransferRow(t)
        }
    }
}

@Composable
private fun TransferRow(t: Transfer) {
    val shape = RoundedCornerShape(15.dp)
    Row(modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
        Box(
            modifier = Modifier
                .size(42.dp)
                .clip(shape)
                .background(Color.White.copy(alpha = 0.08f)),
            contentAlignment = Alignment.Center
        ) {
            Text(t.bankCode.take(2).uppercase(), color = TP.Blue200, fontWeight = FontWeight.ExtraBold, fontSize = 13.sp)
        }
        Spacer(Modifier.width(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(t.accountName, color = Color.White, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
            val extra = if (t.failureReason.isNotEmpty()) " · " + t.failureReason else ""
            Text(
                formatDate(t.createdAt) + " · " + t.accountNumber + extra,
                color = TP.Blue300,
                fontSize = 12.sp,
                maxLines = 2,
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

// ---------------------------------------------------------------- account

@Composable
fun AccountContent(vm: AppViewModel) {
    val m = vm.merchant
    var webhook by remember(m?.webhookUrl) { mutableStateOf(m?.webhookUrl ?: "") }
    var confirmRotate by remember { mutableStateOf(false) }

    GlassCard {
        Text("Appearance", color = TP.Ink, fontSize = 17.sp, fontWeight = FontWeight.Bold)
        Text(if (ThemeState.dark) "Dark mode is on" else "Light mode is on", color = TP.Muted, fontSize = 14.sp)
        Spacer(Modifier.height(12.dp))
        GhostButton(
            if (ThemeState.dark) "Switch to light mode" else "Switch to dark mode",
            { ThemeState.dark = !ThemeState.dark; vm.saveTheme(ThemeState.dark) }
        )
    }
    Spacer(Modifier.height(16.dp))
    GlassCard {
        Text(m?.name ?: "", color = TP.Ink, fontSize = 20.sp, fontWeight = FontWeight.ExtraBold)
        Text(m?.email ?: "", color = TP.Muted, fontSize = 14.sp)
        Spacer(Modifier.height(14.dp))
        KeyValue("Server", vm.serverUrl)
    }
    Spacer(Modifier.height(16.dp))
    GlassCard {
        Text("API keys", color = TP.Ink, fontSize = 17.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(10.dp))
        KeyValue("Public key", m?.publicKey ?: "")
        KeyValue("Secret key", m?.secretKeyHint ?: "")
        KeyValue("Signing secret", m?.webhookSecret ?: "")
        Spacer(Modifier.height(14.dp))
        GhostButton("Rotate secret key", { confirmRotate = true })
    }
    Spacer(Modifier.height(16.dp))
    GlassCard {
        Text("Webhook", color = TP.Ink, fontSize = 17.sp, fontWeight = FontWeight.Bold)
        TechField(webhook, { webhook = it }, "Webhook URL", keyboard = KeyboardType.Uri, placeholder = "https://yourapp.com/webhooks/techet")
        val ae = vm.accountError
        if (ae != null) Text(ae, color = TP.Danger, fontSize = 14.sp, modifier = Modifier.padding(top = 10.dp))
        val an = vm.accountNotice
        if (an != null) Text(an, color = Color(0xFF0B7A53), fontSize = 14.sp, modifier = Modifier.padding(top = 10.dp))
        Spacer(Modifier.height(14.dp))
        PrimaryButton("Save webhook", { vm.saveWebhook(webhook) }, Modifier.fillMaxWidth(), loading = vm.busy > 0)
    }
    Spacer(Modifier.height(16.dp))
    NavyPanel {
        Text("Webhook deliveries", color = Color.White, fontSize = 17.sp, fontWeight = FontWeight.Bold)
        if (vm.deliveries.isEmpty()) {
            Text("No deliveries yet.", color = TP.Blue300, modifier = Modifier.fillMaxWidth().padding(vertical = 22.dp))
        }
        for ((i, d) in vm.deliveries.withIndex()) {
            if (i > 0) Divider()
            Row(modifier = Modifier.fillMaxWidth().padding(vertical = 11.dp), verticalAlignment = Alignment.CenterVertically) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(d.event, color = Color.White, fontWeight = FontWeight.SemiBold)
                    Text(
                        formatDate(d.createdAt) + " · " + d.attempts + " attempt(s)",
                        color = TP.Blue300,
                        fontSize = 12.sp
                    )
                }
                StatusPill(d.status)
            }
        }
    }
    Spacer(Modifier.height(18.dp))
    GhostButton("Sign out", { vm.signOut() }, Modifier.fillMaxWidth())
    Text(
        "Sandbox: no real money moves.",
        color = TP.Muted,
        fontSize = 12.sp,
        modifier = Modifier.fillMaxWidth().padding(top = 14.dp)
    )

    if (confirmRotate) {
        GlassDialog(onDismiss = { confirmRotate = false }) {
            Text("Rotate secret key?", color = TP.Ink, fontSize = 19.sp, fontWeight = FontWeight.Bold)
            Spacer(Modifier.height(8.dp))
            Text("The current key stops working immediately. Update your server before you rotate.", color = TP.Muted, fontSize = 14.sp)
            Spacer(Modifier.height(18.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                GhostButton("Cancel", { confirmRotate = false })
                PrimaryButton("Rotate", { confirmRotate = false; vm.rotateKey() })
            }
        }
    }
}

@Composable
private fun KeyValue(label: String, value: String) {
    Column(modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
        Text(label, color = TP.Muted, fontSize = 12.sp)
        SelectionContainer {
            Text(
                value.ifEmpty { "-" },
                color = TP.Ink,
                fontSize = 13.sp,
                fontFamily = FontFamily.Monospace,
                fontWeight = FontWeight.SemiBold
            )
        }
    }
}

// ---------------------------------------------------------------- dialogs

@Composable
fun CreatePaymentDialog(vm: AppViewModel, onDismiss: () -> Unit) {
    var amount by remember { mutableStateOf("150.00") }
    var email by remember { mutableStateOf("") }
    var url by remember { mutableStateOf<String?>(null) }
    var err by remember { mutableStateOf<String?>(null) }
    var working by remember { mutableStateOf(false) }
    val context = LocalContext.current
    val clipboard = LocalClipboardManager.current
    var copied by remember { mutableStateOf(false) }

    GlassDialog(onDismiss = onDismiss) {
        val link = url
        if (link == null) {
            Text("Create payment link", color = TP.Ink, fontSize = 19.sp, fontWeight = FontWeight.Bold)
            Text("Share it with a customer, or open it to test the checkout.", color = TP.Muted, fontSize = 13.sp)
            TechField(amount, { amount = it }, "Amount (ETB)", keyboard = KeyboardType.Decimal)
            TechField(email, { email = it }, "Customer email (optional)", keyboard = KeyboardType.Email)
            val e = err
            if (e != null) Text(e, color = TP.Danger, fontSize = 14.sp, modifier = Modifier.padding(top = 10.dp))
            Spacer(Modifier.height(16.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                GhostButton("Cancel", onDismiss)
                PrimaryButton(
                    text = "Create link",
                    onClick = {
                        working = true
                        err = null
                        vm.createPaymentLink(amount, email) { u, msg ->
                            working = false
                            url = u
                            err = msg
                        }
                    },
                    modifier = Modifier.weight(1f),
                    loading = working
                )
            }
        } else {
            Text("Payment link ready", color = TP.Ink, fontSize = 19.sp, fontWeight = FontWeight.Bold)
            Text("Valid for 30 minutes.", color = TP.Muted, fontSize = 13.sp)
            Spacer(Modifier.height(10.dp))
            SelectionContainer {
                Text(link, color = TP.Ink, fontSize = 13.sp, fontFamily = FontFamily.Monospace)
            }
            Spacer(Modifier.height(16.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                PrimaryButton("Open", {
                    try {
                        context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(link)))
                    } catch (ex: Exception) {
                        err = "No app can open this link"
                    }
                })
                GhostButton(if (copied) "Copied" else "Copy", {
                    clipboard.setText(AnnotatedString(link))
                    copied = true
                })
                GhostButton("Share", {
                    try {
                        val send = Intent(Intent.ACTION_SEND)
                        send.type = "text/plain"
                        send.putExtra(Intent.EXTRA_TEXT, link)
                        context.startActivity(Intent.createChooser(send, "Share payment link"))
                    } catch (ex: Exception) {
                        err = "Sharing is not available"
                    }
                })
            }
            Spacer(Modifier.height(12.dp))
            GhostButton("Done", onDismiss, Modifier.fillMaxWidth())
        }
    }
}

@Composable
fun TxDetailDialog(t: Tx, onDismiss: () -> Unit) {
    GlassDialog(onDismiss = onDismiss) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("Payment details", color = TP.Ink, fontSize = 19.sp, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
            StatusPill(t.status)
        }
        Spacer(Modifier.height(10.dp))
        DetailRow("Reference", t.txRef)
        DetailRow("Provider ref", t.reference.ifEmpty { "-" })
        DetailRow("Amount", formatMoney(t.amount) + " ETB")
        DetailRow("Fee", formatMoney(t.fee) + " ETB")
        DetailRow("You receive", if (t.status == "success") formatMoney(t.netAmount) + " ETB" else "-")
        DetailRow("Method", if (t.method.isEmpty()) "-" else t.method + " " + t.account)
        DetailRow("Customer", t.email.ifEmpty { "-" })
        if (t.failureReason.isNotEmpty()) DetailRow("Failure", t.failureReason)
        DetailRow("Created", formatDate(t.createdAt))
        Spacer(Modifier.height(16.dp))
        PrimaryButton("Close", onDismiss, Modifier.fillMaxWidth())
    }
}

@Composable
private fun DetailRow(label: String, value: String) {
    Row(modifier = Modifier.fillMaxWidth().padding(vertical = 7.dp)) {
        Text(label, color = TP.Muted, fontSize = 14.sp, modifier = Modifier.weight(1f))
        Text(value, color = TP.Ink, fontSize = 14.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1.4f))
    }
}

@Composable
fun SecretKeyDialog(key: String, onDone: () -> Unit) {
    val clipboard = LocalClipboardManager.current
    var copied by remember { mutableStateOf(false) }
    GlassDialog(onDismiss = {}) {
        Text("Copy your secret key now", color = TP.Ink, fontSize = 19.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(6.dp))
        Text("It is shown only once. Anyone with this key can create payments and withdraw funds on your behalf.", color = TP.Muted, fontSize = 13.sp)
        Spacer(Modifier.height(12.dp))
        val shape = RoundedCornerShape(14.dp)
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .clip(shape)
                .background(Color(0xFFFFF7DF))
                .border(1.dp, Color(0xFFF0D78A), shape)
                .padding(12.dp)
        ) {
            SelectionContainer {
                Text(key, color = TP.Ink, fontSize = 13.sp, fontFamily = FontFamily.Monospace, fontWeight = FontWeight.SemiBold)
            }
        }
        Spacer(Modifier.height(16.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            GhostButton(if (copied) "Copied" else "Copy", {
                clipboard.setText(AnnotatedString(key))
                copied = true
            })
            PrimaryButton("I saved it", onDone, Modifier.weight(1f))
        }
    }
}

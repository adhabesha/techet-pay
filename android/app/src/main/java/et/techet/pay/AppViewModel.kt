package et.techet.pay

import android.app.Application
import android.content.Context
import android.content.SharedPreferences
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import et.techet.pay.data.ApiClient
import et.techet.pay.data.ApiException
import et.techet.pay.data.AuthResult
import et.techet.pay.data.Bank
import et.techet.pay.data.Delivery
import et.techet.pay.data.Merchant
import et.techet.pay.data.Overview
import et.techet.pay.data.Transfer
import et.techet.pay.data.Tx
import et.techet.pay.ui.ThemeState
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

enum class AppTab { Home, Payments, Payouts, Account }

private class Loaded(
    val overview: Overview,
    val txs: List<Tx>,
    val transfers: List<Transfer>,
    val deliveries: List<Delivery>,
    val merchant: Merchant,
    val banks: List<Bank>,
)

class AppViewModel(app: Application) : AndroidViewModel(app) {

    private val prefs: SharedPreferences = app.getSharedPreferences("techet", Context.MODE_PRIVATE)

    var serverUrl by mutableStateOf(prefs.getString("server_url", null) ?: ApiClient.DEFAULT_SERVER)
    var token by mutableStateOf(prefs.getString("token", "") ?: "")
    var merchant by mutableStateOf<Merchant?>(null)
    var booting by mutableStateOf(true)
    var tab by mutableStateOf(AppTab.Home)
    var onboarded by mutableStateOf(prefs.getBoolean("onboarded", false))

    fun finishOnboarding() {
        prefs.edit().putBoolean("onboarded", true).apply()
        onboarded = true
    }

    var overview by mutableStateOf<Overview?>(null)
    var txs by mutableStateOf<List<Tx>>(emptyList())
    var transfers by mutableStateOf<List<Transfer>>(emptyList())
    var deliveries by mutableStateOf<List<Delivery>>(emptyList())
    var banks by mutableStateOf<List<Bank>>(emptyList())

    var busy by mutableStateOf(0)
    var error by mutableStateOf<String?>(null)
    var authError by mutableStateOf<String?>(null)
    var payoutError by mutableStateOf<String?>(null)
    var payoutNotice by mutableStateOf<String?>(null)
    var accountNotice by mutableStateOf<String?>(null)
    var accountError by mutableStateOf<String?>(null)
    var revealedKey by mutableStateOf<String?>(null)

    private val api = ApiClient({ serverUrl }, { token })

    init {
        if (prefs.contains("dark")) ThemeState.dark = prefs.getBoolean("dark", false)
        boot()
    }

    fun saveTheme(dark: Boolean) {
        prefs.edit().putBoolean("dark", dark).apply()
    }

    private fun launchApi(onError: (String) -> Unit = { error = it }, block: suspend () -> Unit) {
        viewModelScope.launch {
            busy += 1
            try {
                block()
            } catch (e: ApiException) {
                if (e.code == 401 && token.isNotEmpty()) {
                    clearSession()
                    authError = "Session expired. Please sign in again."
                } else {
                    onError(e.message ?: "Request failed")
                }
            } catch (e: Exception) {
                onError(e.message ?: "Something went wrong")
            } finally {
                busy -= 1
            }
        }
    }

    private fun boot() {
        if (token.isEmpty()) {
            booting = false
            return
        }
        viewModelScope.launch {
            try {
                merchant = api.me()
                applyLoaded(fetchAll())
            } catch (e: ApiException) {
                if (e.code == 401) {
                    clearSession()
                } else {
                    // server unreachable: keep the session, show the shell with an error
                    error = e.message
                    merchant = Merchant("", "Merchant", "", "", "", "", "")
                }
            } catch (e: Exception) {
                error = e.message
            } finally {
                booting = false
            }
        }
    }

    private suspend fun fetchAll(): Loaded = coroutineScope {
        val o = async { api.overview() }
        val t = async { api.transactions() }
        val r = async { api.transfers() }
        val d = async { api.deliveries() }
        val m = async { api.me() }
        val b = async { if (banks.isEmpty()) api.banks() else banks }
        Loaded(o.await(), t.await(), r.await(), d.await(), m.await(), b.await())
    }

    private fun applyLoaded(l: Loaded) {
        overview = l.overview
        txs = l.txs
        transfers = l.transfers
        deliveries = l.deliveries
        merchant = l.merchant
        banks = l.banks
        error = null
    }

    fun refreshAll() {
        launchApi {
            applyLoaded(fetchAll())
        }
    }

    // ---- auth ----

    fun login(email: String, password: String) {
        authError = null
        launchApi(onError = { authError = it }) {
            startSession(api.login(email.trim(), password))
        }
    }

    fun register(name: String, email: String, password: String) {
        authError = null
        launchApi(onError = { authError = it }) {
            startSession(api.register(name.trim(), email.trim(), password))
        }
    }

    private fun startSession(r: AuthResult) {
        token = r.token
        prefs.edit().putString("token", r.token).apply()
        merchant = r.merchant
        tab = AppTab.Home
        if (!r.secretKey.isNullOrEmpty()) revealedKey = r.secretKey
        refreshAll()
    }

    fun signOut() {
        val old = token
        val url = serverUrl
        clearSession()
        viewModelScope.launch {
            try {
                ApiClient({ url }, { old }).logout()
            } catch (e: Exception) {
                // best effort
            }
        }
    }

    private fun clearSession() {
        token = ""
        prefs.edit().remove("token").apply()
        merchant = null
        overview = null
        txs = emptyList()
        transfers = emptyList()
        deliveries = emptyList()
        revealedKey = null
        error = null
        tab = AppTab.Home
    }

    fun saveServerUrl(raw: String): String? {
        val clean = raw.trim().trimEnd('/')
        if (!(clean.startsWith("http://") || clean.startsWith("https://"))) {
            return "Server address must start with http:// or https://"
        }
        serverUrl = clean
        prefs.edit().putString("server_url", clean).apply()
        return null
    }

    // ---- actions ----

    fun createPaymentLink(amount: String, email: String, onResult: (String?, String?) -> Unit) {
        launchApi(onError = { onResult(null, it) }) {
            val url = api.createTestPayment(amount.trim(), email.trim())
            onResult(url, null)
            refreshAll()
        }
    }

    fun requestPayout(bankCode: String, amount: String, name: String, account: String) {
        payoutError = null
        payoutNotice = null
        launchApi(onError = { payoutError = it }) {
            api.createTransfer(bankCode, amount.trim(), name.trim(), account.trim())
            payoutNotice = "Payout queued. Funds are on hold until the bank confirms."
            refreshAll()
            viewModelScope.launch {
                delay(4000)
                refreshAll()
            }
        }
    }

    fun saveWebhook(url: String) {
        accountNotice = null
        accountError = null
        launchApi(onError = { accountError = it }) {
            api.setWebhook(url.trim())
            accountNotice = "Webhook saved."
            merchant = api.me()
        }
    }

    fun rotateKey() {
        accountNotice = null
        accountError = null
        launchApi(onError = { accountError = it }) {
            val r = api.rotateKey()
            merchant = r.merchant
            revealedKey = r.secretKey
        }
    }
}

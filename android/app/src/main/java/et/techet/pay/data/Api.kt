package et.techet.pay.data

import et.techet.pay.BuildConfig
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

class ApiException(message: String, val code: Int) : Exception(message)

data class Merchant(
    val id: String,
    val name: String,
    val email: String,
    val publicKey: String,
    val secretKeyHint: String,
    val webhookUrl: String,
    val webhookSecret: String,
)

data class SeriesPoint(val date: String, val volume: String)

data class Overview(
    val availableBalance: String,
    val grossVolume: String,
    val netEarned: String,
    val successful: Int,
    val failed: Int,
    val pending: Int,
    val series: List<SeriesPoint>,
    val feePercent: Double,
    val payoutFee: String,
)

data class Tx(
    val txRef: String,
    val reference: String,
    val status: String,
    val amount: String,
    val fee: String,
    val netAmount: String,
    val email: String,
    val method: String,
    val account: String,
    val failureReason: String,
    val createdAt: String,
)

data class Transfer(
    val id: String,
    val reference: String,
    val status: String,
    val amount: String,
    val fee: String,
    val bankCode: String,
    val accountName: String,
    val accountNumber: String,
    val failureReason: String,
    val createdAt: String,
)

data class Bank(val code: String, val name: String, val kind: String)

data class Delivery(
    val id: String,
    val event: String,
    val status: String,
    val attempts: Int,
    val lastError: String,
    val createdAt: String,
)

data class AuthResult(val token: String, val merchant: Merchant, val secretKey: String?)

data class KeyResult(val merchant: Merchant, val secretKey: String)

class ApiClient(private val baseUrlProvider: () -> String, private val tokenProvider: () -> String) {

    private suspend fun call(method: String, path: String, body: JSONObject?, auth: Boolean): JSONObject =
        withContext(Dispatchers.IO) {
            val base = baseUrlProvider().trim().trimEnd('/')
            if (!(base.startsWith("http://") || base.startsWith("https://"))) {
                throw ApiException("Server address must start with http:// or https://", 0)
            }
            val conn = try {
                URL(base + path).openConnection() as HttpURLConnection
            } catch (e: Exception) {
                throw ApiException("Invalid server address", 0)
            }
            try {
                conn.requestMethod = method
                conn.connectTimeout = 30000
                conn.readTimeout = 75000
                conn.setRequestProperty("Accept", "application/json")
                if (auth) conn.setRequestProperty("Authorization", "Bearer " + tokenProvider())
                if (body != null) {
                    conn.doOutput = true
                    conn.setRequestProperty("Content-Type", "application/json")
                    conn.outputStream.use { it.write(body.toString().toByteArray(Charsets.UTF_8)) }
                }
                val code = conn.responseCode
                val stream = if (code in 200..299) conn.inputStream else conn.errorStream
                val text = stream?.bufferedReader(Charsets.UTF_8)?.use { it.readText() } ?: ""
                val json = try {
                    if (text.isBlank()) JSONObject() else JSONObject(text)
                } catch (e: Exception) {
                    JSONObject()
                }
                if (code !in 200..299) {
                    val msg = json.optString("message", "")
                    throw ApiException(if (msg.isNotBlank()) msg else "Request failed (HTTP $code)", code)
                }
                json
            } catch (e: ApiException) {
                throw e
            } catch (e: IOException) {
                throw ApiException("Cannot reach the server. Check your internet connection and the server address.", 0)
            } finally {
                conn.disconnect()
            }
        }

    private fun data(o: JSONObject): JSONObject = o.optJSONObject("data") ?: JSONObject()
    private fun dataArray(o: JSONObject): JSONArray = o.optJSONArray("data") ?: JSONArray()

    private fun merchant(o: JSONObject) = Merchant(
        id = o.optString("id"),
        name = o.optString("name"),
        email = o.optString("email"),
        publicKey = o.optString("public_key"),
        secretKeyHint = o.optString("secret_key_hint"),
        webhookUrl = o.optString("webhook_url"),
        webhookSecret = o.optString("webhook_secret"),
    )

    suspend fun register(name: String, email: String, password: String): AuthResult {
        val b = JSONObject().put("name", name).put("email", email).put("password", password)
        val d = data(call("POST", "/dash/register", b, false))
        return AuthResult(d.optString("token"), merchant(d.optJSONObject("merchant") ?: JSONObject()), d.optString("secret_key"))
    }

    suspend fun login(email: String, password: String): AuthResult {
        val b = JSONObject().put("email", email).put("password", password)
        val d = data(call("POST", "/dash/login", b, false))
        return AuthResult(d.optString("token"), merchant(d.optJSONObject("merchant") ?: JSONObject()), null)
    }

    suspend fun logout() {
        call("POST", "/dash/logout", JSONObject(), true)
    }

    suspend fun me(): Merchant = merchant(data(call("GET", "/dash/me", null, true)))

    suspend fun overview(): Overview {
        val d = data(call("GET", "/dash/overview", null, true))
        val arr = d.optJSONArray("series") ?: JSONArray()
        val series = ArrayList<SeriesPoint>()
        for (i in 0 until arr.length()) {
            val p = arr.optJSONObject(i) ?: continue
            series.add(SeriesPoint(p.optString("date"), p.optString("volume", "0.00")))
        }
        return Overview(
            availableBalance = d.optString("available_balance", "0.00"),
            grossVolume = d.optString("gross_volume", "0.00"),
            netEarned = d.optString("net_earned", "0.00"),
            successful = d.optInt("successful"),
            failed = d.optInt("failed"),
            pending = d.optInt("pending"),
            series = series,
            feePercent = d.optDouble("fee_percent", 0.0),
            payoutFee = d.optString("payout_fee", "0.00"),
        )
    }

    suspend fun transactions(): List<Tx> {
        val arr = dataArray(call("GET", "/dash/transactions", null, true))
        val out = ArrayList<Tx>()
        for (i in 0 until arr.length()) {
            val o = arr.optJSONObject(i) ?: continue
            out.add(
                Tx(
                    txRef = o.optString("tx_ref"),
                    reference = o.optString("reference"),
                    status = o.optString("status"),
                    amount = o.optString("amount", "0.00"),
                    fee = o.optString("fee", "0.00"),
                    netAmount = o.optString("net_amount", "0.00"),
                    email = o.optString("email"),
                    method = o.optString("payment_method"),
                    account = o.optString("account"),
                    failureReason = o.optString("failure_reason"),
                    createdAt = o.optString("created_at"),
                )
            )
        }
        return out
    }

    suspend fun transfers(): List<Transfer> {
        val arr = dataArray(call("GET", "/dash/transfers", null, true))
        val out = ArrayList<Transfer>()
        for (i in 0 until arr.length()) {
            val o = arr.optJSONObject(i) ?: continue
            out.add(
                Transfer(
                    id = o.optString("id"),
                    reference = o.optString("reference"),
                    status = o.optString("status"),
                    amount = o.optString("amount", "0.00"),
                    fee = o.optString("fee", "0.00"),
                    bankCode = o.optString("bank_code"),
                    accountName = o.optString("account_name"),
                    accountNumber = o.optString("account_number"),
                    failureReason = o.optString("failure_reason"),
                    createdAt = o.optString("created_at"),
                )
            )
        }
        return out
    }

    suspend fun createTransfer(bankCode: String, amount: String, accountName: String, accountNumber: String) {
        val b = JSONObject()
            .put("bank_code", bankCode)
            .put("amount", amount)
            .put("account_name", accountName)
            .put("account_number", accountNumber)
        call("POST", "/dash/transfers", b, true)
    }

    suspend fun banks(): List<Bank> {
        val arr = dataArray(call("GET", "/v1/banks", null, false))
        val out = ArrayList<Bank>()
        for (i in 0 until arr.length()) {
            val o = arr.optJSONObject(i) ?: continue
            out.add(Bank(o.optString("code"), o.optString("name"), o.optString("kind")))
        }
        return out
    }

    suspend fun createTestPayment(amount: String, email: String): String {
        val b = JSONObject().put("amount", amount).put("email", email)
        return data(call("POST", "/dash/test-payment", b, true)).optString("checkout_url")
    }

    suspend fun setWebhook(url: String) {
        call("POST", "/dash/webhook", JSONObject().put("url", url), true)
    }

    suspend fun rotateKey(): KeyResult {
        val d = data(call("POST", "/dash/rotate-key", JSONObject(), true))
        return KeyResult(merchant(d.optJSONObject("merchant") ?: JSONObject()), d.optString("secret_key"))
    }

    suspend fun deliveries(): List<Delivery> {
        val arr = dataArray(call("GET", "/dash/deliveries", null, true))
        val out = ArrayList<Delivery>()
        for (i in 0 until arr.length()) {
            val o = arr.optJSONObject(i) ?: continue
            out.add(
                Delivery(
                    id = o.optString("id"),
                    event = o.optString("event"),
                    status = o.optString("status"),
                    attempts = o.optInt("attempts"),
                    lastError = o.optString("last_error"),
                    createdAt = o.optString("created_at"),
                )
            )
        }
        return out
    }

    companion object {
        const val DEFAULT_SERVER = BuildConfig.DEFAULT_SERVER_URL
    }
}

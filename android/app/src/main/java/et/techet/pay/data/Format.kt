package et.techet.pay.data

import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale

/** Splits a decimal string like "12450.75" into ("12,450", "75"). Never uses floating point. */
fun moneyParts(s: String): Pair<String, String> {
    val clean = s.trim().ifEmpty { "0" }
    val neg = clean.startsWith("-")
    val body = clean.removePrefix("-")
    val parts = body.split('.')
    val whole = parts[0].ifEmpty { "0" }.filter { it.isDigit() }.ifEmpty { "0" }
    var frac = if (parts.size > 1) parts[1].filter { it.isDigit() } else ""
    frac = (frac + "00").take(2)
    val sb = StringBuilder()
    for ((i, ch) in whole.withIndex()) {
        if (i > 0 && (whole.length - i) % 3 == 0) sb.append(',')
        sb.append(ch)
    }
    return Pair((if (neg) "-" else "") + sb.toString(), frac)
}

fun formatMoney(s: String): String {
    val p = moneyParts(s)
    return p.first + "." + p.second
}

fun formatDate(iso: String): String {
    return try {
        val t = OffsetDateTime.parse(iso).atZoneSameInstant(ZoneId.systemDefault())
        t.format(DateTimeFormatter.ofPattern("MMM d, HH:mm", Locale.getDefault()))
    } catch (e: Exception) {
        iso
    }
}

/** Parses "1234.50" to a Double for charting only (never for money math). */
fun chartValue(s: String): Double = s.toDoubleOrNull() ?: 0.0

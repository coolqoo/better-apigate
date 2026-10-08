import { useState } from "react";
import stripe from "@/assets/payments/stripe.svg";
import visa from "@/assets/payments/visa.svg";
import mastercard from "@/assets/payments/mastercard.svg";
import amex from "@/assets/payments/amex.svg";
import discover from "@/assets/payments/discover.svg";
import jcb from "@/assets/payments/jcb.svg";
import usdt from "@/assets/payments/usdt.svg";
import usdc from "@/assets/payments/usdc.svg";
import btc from "@/assets/payments/btc.svg";
import eth from "@/assets/payments/eth.svg";
import sol from "@/assets/payments/sol.svg";
import trx from "@/assets/payments/trx.svg";
import bnb from "@/assets/payments/bnb.svg";
import ltc from "@/assets/payments/ltc.svg";

const methods = [
  { id: "stripe", name: "Stripe", icon: stripe, crypto: false },
  { id: "visa", name: "Visa", icon: visa, crypto: false },
  { id: "mastercard", name: "Mastercard", icon: mastercard, crypto: false },
  { id: "amex", name: "AMEX", icon: amex, crypto: false },
  { id: "discover", name: "Discover", icon: discover, crypto: false },
  { id: "jcb", name: "JCB", icon: jcb, crypto: false },
  { id: "usdt", name: "USDT", icon: usdt, crypto: true },
  { id: "usdc", name: "USDC", icon: usdc, crypto: true },
  { id: "btc", name: "Bitcoin", icon: btc, crypto: true },
  { id: "eth", name: "Ethereum", icon: eth, crypto: true },
  { id: "sol", name: "Solana", icon: sol, crypto: true },
  { id: "trx", name: "TRON", icon: trx, crypto: true },
  { id: "bnb", name: "BNB", icon: bnb, crypto: true },
  { id: "ltc", name: "Litecoin", icon: ltc, crypto: true },
];

function PaymentLogo({ method }: { method: (typeof methods)[number] }) {
  return (
    <img
      src={method.icon}
      alt=""
      className={
        method.crypto ? "size-8 shrink-0" : "h-8 w-12 shrink-0 object-contain"
      }
      width={method.crypto ? 32 : 48}
      height={32}
    />
  );
}

export function PayWith({ selected }: { selected: string[] }) {
  const visible = methods.filter((method) => selected.includes(method.id));
  if (!visible.length) return null;
  return (
    <section
      aria-labelledby="pay-with-title"
      className="border-t py-10 md:py-12"
    >
      <div className="mb-6 text-center">
        <h2
          id="pay-with-title"
          className="text-xl font-semibold tracking-tight"
        >
          Pay with
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Flexible ways to fund your wallet.
        </p>
      </div>
      <ul className="mx-auto flex max-w-4xl flex-wrap justify-center gap-3">
        {visible.map((method) => (
          <li
            key={method.id}
            className="flex min-w-28 items-center justify-center gap-2.5 rounded-xl border bg-card px-4 py-3"
          >
            <PaymentLogo method={method} />
            <span className="text-xs font-medium">{method.name}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

export function PaymentMethodPicker({ initial }: { initial: string }) {
  const [selected, setSelected] = useState(initial.split(",").filter(Boolean));
  return (
    <div className="space-y-6 sm:col-span-2">
      <input
        type="hidden"
        name="custom.payment_methods"
        value={selected.join(",")}
      />
      {[false, true].map((crypto) => (
        <fieldset key={String(crypto)}>
          <legend className="mb-3 text-sm font-medium">
            {crypto ? "Cryptocurrency" : "Cards & providers"}
          </legend>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {methods
              .filter((method) => method.crypto === crypto)
              .map((method) => (
                <label
                  key={method.id}
                  className={`flex cursor-pointer items-center gap-3 rounded-xl border p-3 transition-colors ${selected.includes(method.id) ? "border-primary/40 bg-primary/5" : "hover:bg-muted/40"}`}
                >
                  <input
                    type="checkbox"
                    aria-label={`Show ${method.name} on homepage`}
                    checked={selected.includes(method.id)}
                    className="size-4 accent-primary"
                    onChange={(event) =>
                      setSelected((current) =>
                        event.target.checked
                          ? [...current, method.id]
                          : current.filter((id) => id !== method.id),
                      )
                    }
                  />
                  <PaymentLogo method={method} />
                  <span className="text-sm font-medium">{method.name}</span>
                </label>
              ))}
          </div>
        </fieldset>
      ))}
    </div>
  );
}

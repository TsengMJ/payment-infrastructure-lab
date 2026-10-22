import { LoginForm } from "@/app/login-form";
import { loginMerchantAction } from "@/app/actions";
import { MERCHANT_ACCOUNTS } from "@/lib/demo";

export default function LoginPage() {
  return (
    <main>
      <h1>商戶登入</h1>
      <p>這是給人使用的帳號，不是 API key。</p>
      <LoginForm action={loginMerchantAction} accounts={MERCHANT_ACCOUNTS} />
    </main>
  );
}

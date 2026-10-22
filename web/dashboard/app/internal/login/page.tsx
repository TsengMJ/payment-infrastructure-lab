import { loginInternalAction } from "@/app/actions";
import { LoginForm } from "@/app/login-form";
import { INTERNAL_ACCOUNTS } from "@/lib/demo";

export default function InternalLoginPage() {
  return (
    <main>
      <h1>內部人員登入</h1>
      <p>內部帳號與商戶帳號分開，商戶登入不能進入這裡。</p>
      <LoginForm action={loginInternalAction} accounts={INTERNAL_ACCOUNTS} />
    </main>
  );
}

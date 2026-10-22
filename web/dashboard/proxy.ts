import { NextResponse, type NextRequest } from "next/server";

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  if (pathname.startsWith("/api/")) {
    return NextResponse.next();
  }
  if (pathname.startsWith("/internal")) {
    if (pathname === "/internal/login") {
      return NextResponse.next();
    }
    if (!request.cookies.get("internal_session")) {
      return NextResponse.redirect(new URL("/internal/login", request.url));
    }
    return NextResponse.next();
  }
  if (pathname === "/login") {
    return NextResponse.next();
  }
  if (!request.cookies.get("merchant_session")) {
    return NextResponse.redirect(new URL("/login", request.url));
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};

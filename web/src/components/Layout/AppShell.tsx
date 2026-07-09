import { ReactNode } from 'react';
import Sidebar from './Sidebar';

interface Props {
  sessionId: string;
  onResumeSession: (id: string) => void;
  onNewSession: () => void;
  children: ReactNode;
}

export default function AppShell({ sessionId, onResumeSession, onNewSession, children }: Props) {
  return (
    <div className="flex h-full">
      <Sidebar sessionId={sessionId} onResumeSession={onResumeSession} onNewSession={onNewSession} />
      <div className="flex-1 overflow-hidden">{children}</div>
    </div>
  );
}

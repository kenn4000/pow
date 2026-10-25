require 'json'

project = Project.find_by_full_path('kenn/prisoners-of-war')
raise 'Project not found' unless project
admin = User.admins.active.order(:id).first
raise 'Administrator not found' unless admin
public_key = ENV.fetch('POW_PUBLIC_KEY').strip

User.transaction do
  user = User.find_by_username('pow-workspace')
  unless user
    result = Users::CreateService.new(admin, {
      username: 'pow-workspace',
      name: 'Prisoners of War Workspace',
      email: 'pow-workspace@markdownthing.com',
      organization_id: project.namespace.organization_id,
      force_random_password: true,
      password_automatically_set: true,
      skip_confirmation: true,
      external: true,
      admin: false,
      can_create_group: false,
      projects_limit: 0
    }).execute
    raise result.message unless result.success?
    user = result.payload.fetch(:user)
  end
  raise 'Existing user has an unexpected identity' unless user.email == 'pow-workspace@markdownthing.com' && !user.admin?

  key = user.keys.find_by(key: public_key)
  key ||= user.keys.create!(title: 'Shared Prisoners of War development workspace', key: public_key)
  member = project.project_members.find_by(user_id: user.id)
  if member
    member.update!(access_level: Gitlab::Access::DEVELOPER) if member.access_level < Gitlab::Access::DEVELOPER
  else
    member = project.add_developer(user)
    raise member.errors.full_messages.join(', ') unless member.persisted?
  end

  puts({
    username: user.username,
    user_id: user.id,
    email: user.email,
    state: user.state,
    external: user.external?,
    project: project.full_path,
    access_level: member.access_level,
    key_id: key.id,
    fingerprint: key.fingerprint_sha256
  }.to_json)
end
